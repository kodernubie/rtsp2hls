# rtsp2hls
Golang module to convert RTSP (eq. IP CCTV video stream) to HLS so the video can be viewed via web browser. Pure golang, no c/c++ dependency

This module is a rewrite from https://github.com/deepch/RTSPtoHLS , because I need a import friendly module, not an example project that need to copy and paste every time.

Currently only support H.264 video without audio (audio disabled by default).

Usage, first get the module using :

```
go get github.com/kodernubie/rtsp2hls
```


To open rtsp stream, use following code :

```
    import (
        rtsp2hls "github.com/kodernubie/rtsp2hls"
    )

    stream, err := rtsp2hls.open("rtsp://server.com/stream1")

    if err == nil {
        
        fmt.println("Success opening stream with id :", stream.id)
    }

```



To generate playlist file that can be used by video player, :

```
    import (
	    "github.com/gofiber/fiber/v2"
	    rtsp2hls "github.com/kodernubie/rtsp2hls"
    )
    
    app := fiber.New()

    // called by videoplayer : http://server.com/stream/[streamId]/index.m3u8
    app.Get("/stream/:streamId/index.m3u8", function(c *fiber.Context) {

        streamId := c.Param("streamId")
        stream := rtsp2hls.get(streamId)

        c.Response().Header.Add("Content-Type", "application/x-mpegURL")

        // provide your custom base url for segment file
        // example if you provide base url : http://server.com/stream/[streamId]/segment/
        // then your media file will be :
        // http://server.com/stream/[streamId]/segment/1.ts
        // http://server.com/stream/[streamId]/segment/2.ts
        // http://server.com/stream/[streamId]/segment/3.ts
        c.sendString(stream.Playlist("http://server.com/stream/" + streamId + "/segment/"))  
    })

```


Then implement api that return segment file binary data :

```
    import (
	    "github.com/gofiber/fiber/v2"
	    rtsp2hls "github.com/kodernubie/rtsp2hls"
    )
    
    app := fiber.New()

    // called by videoplayer : http://server.com/stream/[streamId]/segment/1.ts 
    app.Get("/stream/:streamId/segment/:segmentId", function(c *fiber.Context) {

        streamId := c.Param("streamId")
        stream := rtsp2hls.get(streamId)

        c.Response().Header.Add("Content-Type", "video/mp2t")

        segmentId := c.Params("segmentId")
        segmentId = mediaId[0 : len(segmentId)-3]

        byt, _ := stream.Segment(segmentId)

        c.Response().Header.Add("Content-Type", "video/mp2t")
        return c.SendStream(bytes.NewReader(byt)) 
    })

```


Please check "example" folder for complete example of RTSP 2 HLS server.