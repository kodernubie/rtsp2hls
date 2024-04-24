package main

import (
	"bytes"
	"fmt"
	"net/http"

	"github.com/gofiber/fiber/v2"
	rtsp2hls "github.com/kodernubie/rtsp2hls"
)

type OpenReq struct {
	URL string `json:"url"`
}

type Result struct {
	Code    int         `json:"url"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

func main() {

	app := fiber.New()

	app.Static("/", "./www")

	app.Post("/stream", openStream)
	app.Get("/stream/:id/index.m3u8", getPlaylist)
	app.Get("/stream/:id/media/:mediaId", getMedia)

	app.Listen(":3000")
}

func sendError(c *fiber.Ctx, msg string) error {

	return c.JSON(Result{
		Code:    http.StatusBadRequest,
		Message: msg,
	})
}

func sendResult(c *fiber.Ctx, data interface{}) error {

	return c.JSON(Result{
		Code:    http.StatusOK,
		Message: "success",
		Data:    data,
	})
}

func openStream(c *fiber.Ctx) error {

	req := OpenReq{}
	err := c.BodyParser(&req)

	if err != nil {
		return sendError(c, "invalid request "+err.Error())
	}

	stream, err := rtsp2hls.Open(req.URL)

	if err != nil {
		return sendError(c, "stream open error "+err.Error())
	}

	return sendResult(c, stream.ID)
}

func getPlaylist(c *fiber.Ctx) error {

	stream := rtsp2hls.Get(c.Params("id"))

	if stream == nil {

		return c.SendStatus(404)
	}

	url := string(c.Request().URI().FullURI())
	url = url[0 : len(url)-11]

	c.Response().Header.Add("Content-Type", "application/x-mpegURL")
	return c.SendString(stream.PlayList(url + "/media/"))
}

func getMedia(c *fiber.Ctx) error {

	stream := rtsp2hls.Get(c.Params("id"))

	if stream == nil {
		return c.SendStatus(http.StatusNotFound)
	}

	mediaId := c.Params("mediaId")
	mediaId = mediaId[0 : len(mediaId)-3]

	fmt.Println("====>", stream.ID, "===>", mediaId)
	byt, err := stream.Segment(mediaId)

	if err != nil {
		fmt.Println("Error :", err)
		return c.SendStatus(http.StatusNotFound)
	}

	c.Response().Header.Add("Content-Type", "video/mp2t")
	return c.SendStream(bytes.NewReader(byt))
}
