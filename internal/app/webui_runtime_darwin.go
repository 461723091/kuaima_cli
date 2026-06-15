//go:build darwin && cgo

package app

/*
#cgo darwin CFLAGS: -x objective-c
#cgo darwin LDFLAGS: -framework Cocoa -framework CoreFoundation

#import <Cocoa/Cocoa.h>
#import <CoreFoundation/CoreFoundation.h>
#import <stdlib.h>

extern void goWebUITrayReady(void);
extern void goWebUITrayOpen(void);
extern void goWebUITrayExit(void);

@interface KuaimaTrayTarget : NSObject
@end

@implementation KuaimaTrayTarget
- (void)openClicked:(id)sender {
	(void)sender;
	goWebUITrayOpen();
}

- (void)exitClicked:(id)sender {
	(void)sender;
	goWebUITrayExit();
}
@end

static CFRunLoopRef trayRunLoop = NULL;
static NSStatusItem *trayStatusItem = nil;
static KuaimaTrayTarget *trayTarget = nil;

static NSImage *loadTrayImage(const void *bytes, size_t length) {
	if (bytes == NULL || length == 0) {
		return [NSImage imageNamed:NSImageNameApplicationIcon];
	}
	NSData *data = [NSData dataWithBytes:bytes length:length];
	NSImage *image = [[NSImage alloc] initWithData:data];
	if (image == nil) {
		image = [NSImage imageNamed:NSImageNameApplicationIcon];
	}
	return image;
}

void webui_tray_stop(void) {
	if (trayRunLoop != NULL) {
		CFRunLoopStop(trayRunLoop);
	}
}

void webui_tray_run(const void *iconBytes, size_t iconLen) {
	@autoreleasepool {
		[NSApplication sharedApplication];
		[NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];

		trayRunLoop = CFRunLoopGetCurrent();
		if (trayRunLoop != NULL) {
			CFRetain(trayRunLoop);
		}

		trayTarget = [KuaimaTrayTarget new];
		trayStatusItem = [[NSStatusBar systemStatusBar] statusItemWithLength:NSVariableStatusItemLength];
		if (trayStatusItem != nil) {
			NSStatusBarButton *button = trayStatusItem.button;
			NSImage *image = loadTrayImage(iconBytes, iconLen);
			if (button != nil && image != nil) {
				image.template = YES;
				button.image = image;
				button.imagePosition = NSImageOnly;
				button.toolTip = @"快马 AI WebUI";
			}

			NSMenu *menu = [[NSMenu alloc] initWithTitle:@"Kuaima CLI"];
			NSMenuItem *openItem = [[NSMenuItem alloc] initWithTitle:@"打开 WebUI" action:@selector(openClicked:) keyEquivalent:@""];
			[openItem setTarget:trayTarget];
			[menu addItem:openItem];
			[menu addItem:[NSMenuItem separatorItem]];
			NSMenuItem *exitItem = [[NSMenuItem alloc] initWithTitle:@"退出" action:@selector(exitClicked:) keyEquivalent:@""];
			[exitItem setTarget:trayTarget];
			[menu addItem:exitItem];
			trayStatusItem.menu = menu;
		}

		goWebUITrayReady();
		[NSApp run];

		if (trayStatusItem != nil) {
			[[NSStatusBar systemStatusBar] removeStatusItem:trayStatusItem];
			trayStatusItem = nil;
		}
		trayTarget = nil;
		if (trayRunLoop != NULL) {
			CFRelease(trayRunLoop);
			trayRunLoop = NULL;
		}
	}
}
*/
import "C"

import (
	"runtime"
	"sync"
	"unsafe"
)

type webUIRuntimeOptions struct {
	HideConsole bool
}

type webUITrayState struct {
	url  string
	stop func()
}

var (
	trayStateMu sync.Mutex
	trayState   *webUITrayState
	trayReadyMu sync.Mutex
	trayReady   chan struct{}
)

func hideWebUIConsole() {}

func startWebUITray(rawURL string, stop func()) func() {
	done := make(chan struct{})
	ready := make(chan struct{})
	trayReadyMu.Lock()
	trayReady = ready
	trayReadyMu.Unlock()

	go runWebUITray(rawURL, stop, ready, done)
	<-ready

	return func() {
		trayStateMu.Lock()
		current := trayState
		trayStateMu.Unlock()
		if current != nil && current.stop != nil {
			current.stop()
		}
		<-done
	}
}

func runWebUITray(rawURL string, stop func(), ready, done chan struct{}) {
	defer close(done)
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	once := sync.Once{}
	trayStop := func() {
		once.Do(func() {
			C.webui_tray_stop()
			if stop != nil {
				stop()
			}
		})
	}

	trayStateMu.Lock()
	trayState = &webUITrayState{url: rawURL, stop: trayStop}
	trayStateMu.Unlock()
	defer func() {
		trayStateMu.Lock()
		if trayState != nil && trayState.stop == trayStop {
			trayState = nil
		}
		trayStateMu.Unlock()
	}()

	var iconPtr unsafe.Pointer
	var iconLen C.size_t
	if logo, err := webUIAssets.ReadFile("webui/logo.png"); err == nil && len(logo) > 0 {
		iconPtr = C.CBytes(logo)
		iconLen = C.size_t(len(logo))
		defer C.free(iconPtr)
	}

	C.webui_tray_run(iconPtr, iconLen)
}

//export goWebUITrayReady
func goWebUITrayReady() {
	trayReadyMu.Lock()
	ready := trayReady
	trayReady = nil
	trayReadyMu.Unlock()
	if ready != nil {
		close(ready)
	}
}

//export goWebUITrayOpen
func goWebUITrayOpen() {
	openTrayWebUI()
}

//export goWebUITrayExit
func goWebUITrayExit() {
	stopTrayWebUI()
}

func openTrayWebUI() {
	trayStateMu.Lock()
	rawURL := ""
	if trayState != nil {
		rawURL = trayState.url
	}
	trayStateMu.Unlock()
	if rawURL != "" {
		_ = openBrowser(rawURL)
	}
}

func stopTrayWebUI() {
	trayStateMu.Lock()
	stop := func() {}
	if trayState != nil && trayState.stop != nil {
		stop = trayState.stop
	}
	trayStateMu.Unlock()
	stop()
}
