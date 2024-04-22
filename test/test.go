package main

import (
	"fmt"

	rtsp2hls "github.com/kodernubie/rtsp2hls"
)

func main() {

	stream, err := rtsp2hls.Open("rtsp://admincctv:admin123@192.168.0.101/stream1")

	if err != nil {
		fmt.Println("Error opening stream :", err)
		return
	}

	stream.Segment("stream/01HTXY957AYZCTRAHT8XPE27ZT/6.ts")

	// count := 1
	// for {

	// 	count++
	// 	if count%10 == 0 {
	// 		fmt.Println("===================")
	// 		fmt.Println(stream.PlayList("stream/" + stream.ID + "/"))
	// 	}

	// 	time.Sleep(time.Second)
	// }
}
