//go:build darwin && cgo

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
