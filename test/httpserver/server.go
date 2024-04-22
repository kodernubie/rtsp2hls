package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

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

func sendError(w http.ResponseWriter, msg string) {

	payload, _ := json.Marshal(Result{
		Code:    http.StatusBadRequest,
		Message: msg,
	})

	w.WriteHeader(http.StatusBadRequest)
	w.Write(payload)
}

func sendResult(w http.ResponseWriter, data interface{}) {

	payload, _ := json.Marshal(Result{
		Code:    http.StatusOK,
		Message: "success",
		Data:    data,
	})

	w.Write(payload)
}

func main() {

	mux := http.NewServeMux()
	mux.HandleFunc("/index.html", func(w http.ResponseWriter, r *http.Request) {

		content, _ := os.ReadFile("./index.html")
		w.Write(content)
	})

	mux.HandleFunc("GET /stream/{id}", func(w http.ResponseWriter, r *http.Request) {

		id := r.PathValue("id")

		fmt.Println("ID : ", id)
		sendResult(w, "OK "+id)
	})

	// open new stream, payload :
	// {
	// 	"url" : "rstp://user:pass@address/[nameofstream]"
	// }
	mux.HandleFunc("/stream", func(w http.ResponseWriter, r *http.Request) {

		if r.Method == "POST" {

			payload, err := io.ReadAll(r.Body)

			if err != nil {
				fmt.Println("error 1 :", err)
				sendError(w, err.Error())
				return
			}

			fmt.Println("req payload :", string(payload))

			req := OpenReq{}
			err = json.Unmarshal(payload, &req)

			if err != nil {
				fmt.Println("error 2 :", err)
				sendError(w, err.Error())
				return
			}

			stream, err := rtsp2hls.Open(req.URL)

			if err != nil {
				fmt.Println("error 3 :", err)
				sendError(w, err.Error())
				return
			}

			sendResult(w, stream.ID)
		} else {
			sendError(w, "Not supported")
			return
		}
	})

	fmt.Println("Server started at 8090")
	http.ListenAndServe(":8090", nil)
}
