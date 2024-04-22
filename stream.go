package rstp2hls

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/deepch/vdk/av"
	"github.com/deepch/vdk/format/rtspv2"
	"github.com/deepch/vdk/format/ts"
)

// state
var (
	RUNNING = "running"
	STOPPED = "stopped"
)

// event
var (
	EVT_ERROR   = "error"
	EVT_STOPPED = "stopped"
)

var MAX_BUFFER = 6

type StreamEvent func(stream *Stream, event string)

type Stream struct {
	ID               string
	URL              string
	LastError        error
	client           *rtspv2.RTSPClient
	State            string
	hlsSegmentNumber int
	hlsSegmentBuffer map[int]Segment
	stopChan         chan string
	mutex            sync.Mutex
	hasMax           bool
	OnEvent          StreamEvent
}

type Segment struct {
	dur  time.Duration
	data []*av.Packet
}

func (o *Stream) addHLS(val []*av.Packet, dur time.Duration) {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	o.hlsSegmentNumber++
	o.hlsSegmentBuffer[o.hlsSegmentNumber] = Segment{data: val, dur: dur}

	if o.hasMax {
		print("should have delete")
		// delete(o.hlsSegmentBuffer, o.hlsSegmentNumber-MAX_BUFFER-1)
	} else {
		o.hasMax = len(o.hlsSegmentBuffer) >= MAX_BUFFER
	}
}

func (o *Stream) loop() {

	if o.client == nil {
		o.State = STOPPED
		return
	}

	defer func() {

		if o.client != nil {
			o.State = STOPPED
			o.client.Close()
		}
	}()

	keyTest := time.NewTimer(20 * time.Second)
	var preKeyTS = time.Duration(0)
	var Seq []*av.Packet

	for {

		select {
		case <-o.stopChan:
			fmt.Println("stop")
			o.State = STOPPED

			if o.OnEvent != nil {
				o.OnEvent(o, EVT_STOPPED)
			}
			return
		case <-keyTest.C:
			log.Println("Stream stopped because no video received", o.ID)
			o.LastError = errors.New("Stream stopped because no video received")

			o.State = STOPPED

			if o.OnEvent != nil {
				o.OnEvent(o, EVT_ERROR)
			}
			return
		case signals := <-o.client.Signals:
			fmt.Println("signal :", signals)
			switch signals {
			case rtspv2.SignalCodecUpdate:
				log.Println("codec update", o.ID)
			case rtspv2.SignalStreamRTPStop:
				log.Println("rtsp stopped", o.ID)

				o.State = STOPPED

				if o.OnEvent != nil {
					o.OnEvent(o, EVT_STOPPED)
				}
				return
			}
		case packetAV := <-o.client.OutgoingPacketQueue:
			if packetAV.IsKeyFrame {
				fmt.Println("keyframe")
				keyTest.Reset(20 * time.Second)
				if preKeyTS > 0 {
					o.addHLS(Seq, packetAV.Time-preKeyTS)
					Seq = []*av.Packet{}
				}
				preKeyTS = packetAV.Time
			}
			Seq = append(Seq, packetAV)
		}
	}
}

func (o *Stream) start() error {

	o.LastError = nil
	o.hlsSegmentBuffer = map[int]Segment{}

	var err error
	o.client, err = rtspv2.Dial(rtspv2.RTSPClientOptions{URL: o.URL, DisableAudio: false, DialTimeout: 3 * time.Second, ReadWriteTimeout: 3 * time.Second, Debug: false})

	if err != nil {
		o.LastError = err
		return err
	}

	if o.stopChan == nil {
		o.stopChan = make(chan string)
	}

	go func() {
		o.State = RUNNING
		o.loop()
	}()

	return nil
}

func (o *Stream) Stop() error {

	if o.State == STOPPED || o.stopChan == nil {
		return errors.New("stream is not started")
	}

	o.stopChan <- STOPPED

	return nil
}

// segement can be acessed by :
// [baseURL]/[segmentno].ts
// example for base url "media/stream1/" the segment file will be
// media/stream1/1.ts
// media/stream1/2.ts
// media/stream1/3.ts
func (o *Stream) PlayList(baseURL string) string {

	fmt.Println("start Playlist")
	timeCount := 0
	for len(o.hlsSegmentBuffer) < 1 {

		timeCount++

		if timeCount > 10 {
			break
		}

		time.Sleep(time.Second)
	}

	o.mutex.Lock()
	defer o.mutex.Unlock()

	var out string
	var keys []int
	for k := range o.hlsSegmentBuffer {
		keys = append(keys, k)
	}

	sort.Ints(keys)

	out += `#EXTM3U
#EXT-X-TARGETDURATION:4
#EXT-X-VERSION:4
#EXT-X-MEDIA-SEQUENCE:` + strconv.Itoa(keys[0]) + `
`
	var count int
	for _, i := range keys {
		count++
		out += `#EXTINF:` + strconv.FormatFloat(o.hlsSegmentBuffer[i].dur.Seconds(), 'f', 1, 64) + `,
` + baseURL + strconv.Itoa(i) + `.ts
`

	}

	return out
}

func (o *Stream) Segment(segmentUrl string) ([]byte, error) {

	pos := strings.LastIndex(segmentUrl, "/")
	name := segmentUrl[pos+1:]
	name = name[:len(name)-3]

	ret := bytes.NewBuffer([]byte{})
	Muxer := ts.NewMuxer(ret)
	err := Muxer.WriteHeader(o.client.CodecData)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	Muxer.PaddingToMakeCounterCont = true
	seqData, exist := o.hlsSegmentBuffer[stringToInt(name)]
	if !exist {
		log.Println("segement not exist :", name)
		return nil, errors.New("segment not exist :" + name)
	}

	if len(seqData.data) == 0 {
		log.Println(err)
		return nil, errors.New("empty segement" + name)
	}

	for _, v := range seqData.data {
		v.CompositionTime = 1
		err = Muxer.WritePacket(*v)
		if err != nil {
			log.Println(err)
			return nil, err
		}
	}
	err = Muxer.WriteTrailer()
	if err != nil {
		log.Println(err)
		return nil, err
	}

	return ret.Bytes(), nil
}

func stringToInt(val string) int {
	i, err := strconv.Atoi(val)
	if err != nil {
		return 0
	}
	return i
}
