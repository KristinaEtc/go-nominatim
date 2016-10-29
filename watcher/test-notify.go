package main
import ("github.com/mqu/go-notify")

func main() {
	notify.Init("Hello world")
	hello := notify.NotificationNew("Hello World!", "This is an example notification.","dialog-information")
	hello.Show()
}