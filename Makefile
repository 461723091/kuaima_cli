
build:
	go build -buildvcs=false -o dist/kuaima_cli.exe

run:
	dist/kuaima_cli.exe

test:
	dist/kuaima_cli.exe 你是什么模型