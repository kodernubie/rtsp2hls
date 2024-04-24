package rstp2hls

import (
	"github.com/oklog/ulid/v2"
)

var streams = map[string]*Stream{}

func Open(url string) (*Stream, error) {

	var existing = GetByURL(url)

	if existing != nil {

		if existing.State == STOPPED {
			existing.start()
		}

		return existing, nil
	}

	newStream := &Stream{
		ID:  ulid.Make().String(),
		URL: url,
	}

	err := newStream.start()

	if err != nil {
		return nil, err
	}

	streams[newStream.ID] = newStream
	return newStream, nil
}

func Get(id string) *Stream {

	var existing, _ = streams[id]

	return existing
}

func GetByURL(url string) *Stream {

	var existing *Stream

	for _, stream := range streams {
		if stream.URL == url {
			existing = stream
			break
		}
	}

	return existing
}
