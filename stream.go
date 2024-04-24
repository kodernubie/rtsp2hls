package rstp2hls

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"sort"
	"strconv"
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
	needReconnect    bool
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

func (o *Stream) addSegment(val []*av.Packet, dur time.Duration) {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	o.hlsSegmentNumber++
	o.hlsSegmentBuffer[o.hlsSegmentNumber] = Segment{data: val, dur: dur}

	if o.hasMax {
		delete(o.hlsSegmentBuffer, o.hlsSegmentNumber-MAX_BUFFER-1)
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

		r := recover()

		if r != nil {
			log.Println("recover from panic :", r)
		}

		if o.client != nil {
			o.State = STOPPED
			o.client.Close()
		}
	}()

	keyTest := time.NewTimer(20 * time.Second)
	var preKeyTS = time.Duration(0)
	var Seq []*av.Packet

	o.needReconnect = false

	for {

		select {
		case <-o.stopChan:
			log.Println("stop")
			o.State = STOPPED
			return
		case <-keyTest.C:
			log.Println("Stream stopped because no video received", o.ID)
			o.LastError = errors.New("Stream stopped because no video received")

			o.State = STOPPED
			return
		case signals := <-o.client.Signals:
			log.Println("signal :", signals)
			switch signals {
			case rtspv2.SignalCodecUpdate:
				log.Println("codec update", o.ID)
			case rtspv2.SignalStreamRTPStop:
				log.Println("rtsp stopped", o.ID)

				o.needReconnect = true
				return
			}
		case packetAV := <-o.client.OutgoingPacketQueue:
			if packetAV.IsKeyFrame {
				// log.Println("keyframe")
				keyTest.Reset(20 * time.Second)
				if preKeyTS > 0 {
					o.addSegment(Seq, packetAV.Time-preKeyTS)
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
	o.client, err = rtspv2.Dial(rtspv2.RTSPClientOptions{URL: o.URL, DisableAudio: true, DialTimeout: 3 * time.Second, ReadWriteTimeout: 3 * time.Second, Debug: false})

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

		if o.needReconnect {
			numtry := 1

			log.Println("Reconnect :", o.ID, ", try ", numtry)
			var err error = nil

			for numtry < 20 {

				time.Sleep(time.Second)
				err = o.start()

				if err == nil {
					break
				}

				numtry++
			}

			if err != nil && o.OnEvent != nil {
				o.OnEvent(o, EVT_STOPPED)
			}
		} else {
			o.State = STOPPED

			if o.OnEvent != nil {
				o.OnEvent(o, EVT_STOPPED)
			}
		}
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

func (o *Stream) PlayList(baseURL string) (ret string) {

	defer func() {

		r := recover()

		if r != nil {
			ret = ""
		}
	}()

	timeCount := 0
	for len(o.hlsSegmentBuffer) < MAX_BUFFER {

		timeCount++

		if timeCount > 12 {
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

func (o *Stream) Segment(id string) (ret []byte, retErr error) {

	defer func() {

		r := recover()

		if r != nil {
			ret = nil
			retErr = fmt.Errorf("error : %s", r)
		}
	}()

	retBuff := bytes.NewBuffer([]byte{})
	Muxer := ts.NewMuxer(retBuff)
	err := Muxer.WriteHeader(o.client.CodecData)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	Muxer.PaddingToMakeCounterCont = true
	seqData, exist := o.hlsSegmentBuffer[stringToInt(id)]
	if !exist {
		log.Println("segement not exist :", id)
		return nil, errors.New("segment not exist :" + id)
	}

	if len(seqData.data) == 0 {
		log.Println("empty segment " + id)
		return nil, errors.New("empty segement" + id)
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

	return retBuff.Bytes(), nil
}

func stringToInt(val string) int {
	i, err := strconv.Atoi(val)
	if err != nil {
		return 0
	}
	return i
}
