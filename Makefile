
build:
	go build -buildvcs=false -o dist/kuaima_cli.exe

build_skill:
	powershell -NoProfile -ExecutionPolicy Bypass -File scripts/build_skill.ps1

run:
	dist/kuaima_cli.exe

test:
	dist/kuaima_cli.exe 你是什么模型

test_stream:
	dist/kuaima_cli.exe -stream 给我背诵一下滕王阁序

#查询余额
balance:
	dist/kuaima_cli.exe balance

#充值
recharge:
	dist/kuaima_cli.exe recharge

#加上-stream参数，可以避免1min超时错误 
#一般生图时间 1k约80s， 2k约120s， 4k约180s
test_img:
	dist/kuaima_cli.exe -stream -image-generation -file https://weixin.uctphp.com/static/images/ailogo.png 帮我画张快马AI cli的宣传海报，便捷的生图工具，支持gpt-image-2等最新模型,可接入各种agent智能体，参考图是logo

#指定图片大小 
#1k: 1024x1024 1024x1536 1536x1024
#2k: 2048x2048 1440x2560 2560x1440 
#4k: 2880x2880 2160x3840 3840x2160
test_img2:
	dist/kuaima_cli.exe -image-size 1024x1024 -stream -image-generation -file https://weixin.uctphp.com/static/images/ailogo.png 帮我画张快马AI cli的宣传海报，便捷的生图工具，支持gpt-image-2等最新模型,可接入各种agent智能体，参考图是快马AI的logo

#指定生图模型 数量无效
# gpt-image-2 gpt-image-1.5 gpt-image-1 dalle-e-3 dalle-e-2
test_img3:
	dist/kuaima_cli.exe -log log.log -image-model gpt-image-1.5 -image-count 2 -image-size 1024x1024 -stream -image-generation -file https://weixin.uctphp.com/static/images/ailogo.png 帮我画张快马AI cli的宣传海报，便捷的生图工具，支持gpt-image-2等最新模型,可接入各种agent智能体，参考图是快马AI的logo
