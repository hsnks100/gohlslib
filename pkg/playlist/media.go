package playlist

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/bluenviron/gohlslib/pkg/playlist/primitives"
)

// MediaStart is a EXT-X-START tag.
type MediaStart = MultivariantStart

// MediaPlaylistType is a EXT-X-PLAYLIST-TYPE value.
type MediaPlaylistType string

// standard values
const (
	MediaPlaylistTypeEvent = "EVENT"
	MediaPlaylistTypeVOD   = "VOD"
)

// Media is a media playlist.
type Media struct {
	// #EXT-X-VERSION
	// required
	Version int

	// #EXT-X-INDEPENDENT-SEGMENTS
	IndependentSegments bool

	// #EXT-X-START
	Start *MediaStart

	// #EXT-X-ALLOWCACHE
	// removed since v7
	AllowCache *bool

	// #EXT-X-TARGETDURATION
	// required
	TargetDuration int

	// #EXT-X-SERVER-CONTROL
	ServerControl *MediaServerControl

	// #EXT-X-PART-INF
	PartInf *MediaPartInf

	// #EXT-X-MEDIA-SEQUENCE
	// required
	MediaSequence int

	// #EXT-X-DISCONTINUITY-SEQUENCE
	DiscontinuitySequence *int

	// #EXT-X-PLAYLIST-TYPE
	PlaylistType *MediaPlaylistType

	// #EXT-X-MAP
	Map *MediaMap

	// #EXT-X-SKIP
	Skip *MediaSkip

	// segments
	// at least one is required
	Segments []*MediaSegment

	// #EXT-X-PART
	Parts []*MediaPart

	// #EXT-X-PRELOAD-HINT
	PreloadHint *MediaPreloadHint

	// #EXT-X-ENDLIST
	Endlist bool
}

func (m Media) isPlaylist() {}

// Unmarshal decodes the playlist.
func (m *Media) Unmarshal(buf []byte) error {
	r := bufio.NewReader(bytes.NewReader(buf))

	err := primitives.HeaderUnmarshal(r)
	if err != nil {
		return err
	}

	curSegment := &MediaSegment{}

	for {
		line, err := r.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		line = primitives.RemoveReturn(line)

		switch {
		case strings.HasPrefix(line, "#EXT-X-VERSION:"):
			line = line[len("#EXT-X-VERSION:"):]

			tmp, err := strconv.ParseInt(line, 10, 64)
			if err != nil {
				return err
			}
			m.Version = int(tmp)

			if m.Version > maxSupportedVersion {
				return fmt.Errorf("unsupported HLS version (%d)", m.Version)
			}

		case strings.HasPrefix(line, "#EXT-X-INDEPENDENT-SEGMENTS"):
			m.IndependentSegments = true

		case strings.HasPrefix(line, "#EXT-X-START:"):
			line = line[len("#EXT-X-START:"):]

			m.Start = &MultivariantStart{}
			err := m.Start.unmarshal(line)
			if err != nil {
				return err
			}

		case strings.HasPrefix(line, "#EXT-X-ALLOW-CACHE:"):
			line = line[len("#EXT-X-ALLOW-CACHE:"):]

			v := (line == "YES")
			m.AllowCache = &v

		case strings.HasPrefix(line, "#EXT-X-TARGETDURATION:"):
			line = line[len("#EXT-X-TARGETDURATION:"):]

			tmp, err := strconv.ParseInt(line, 10, 64)
			if err != nil {
				return err
			}
			m.TargetDuration = int(tmp)

		case strings.HasPrefix(line, "#EXT-X-SERVER-CONTROL:"):
			line = line[len("#EXT-X-SERVER-CONTROL:"):]

			m.ServerControl = &MediaServerControl{}
			err := m.ServerControl.unmarshal(line)
			if err != nil {
				return err
			}

		case strings.HasPrefix(line, "#EXT-X-PART-INF:"):
			line = line[len("#EXT-X-PART-INF:"):]

			m.PartInf = &MediaPartInf{}
			err = m.PartInf.unmarshal(line)
			if err != nil {
				return err
			}

		case strings.HasPrefix(line, "#EXT-X-MEDIA-SEQUENCE:"):
			line = line[len("#EXT-X-MEDIA-SEQUENCE:"):]

			tmp, err := strconv.ParseInt(line, 10, 64)
			if err != nil {
				return err
			}

			m.MediaSequence = int(tmp)

		case strings.HasPrefix(line, "#EXT-X-DISCONTINUITY-SEQUENCE:"):
			line = line[len("#EXT-X-DISCONTINUITY-SEQUENCE:"):]

			tmp, err := strconv.ParseInt(line, 10, 64)
			if err != nil {
				return err
			}

			v := int(tmp)
			m.DiscontinuitySequence = &v

		case strings.HasPrefix(line, "#EXT-X-PLAYLIST-TYPE:"):
			line = line[len("#EXT-X-PLAYLIST-TYPE:"):]

			v := MediaPlaylistType(line)
			if v != MediaPlaylistTypeEvent &&
				v != MediaPlaylistTypeVOD {
				return fmt.Errorf("invalid playlist type: %s", v)
			}
			m.PlaylistType = &v

		case strings.HasPrefix(line, "#EXT-X-MAP:"):
			line = line[len("#EXT-X-MAP:"):]

			m.Map = &MediaMap{}
			err = m.Map.unmarshal(line)
			if err != nil {
				return err
			}

		case strings.HasPrefix(line, "#EXT-X-SKIP:"):
			line = line[len("#EXT-X-SKIP:"):]

			m.Skip = &MediaSkip{}
			err = m.Skip.unmarshal(line)
			if err != nil {
				return err
			}

		case strings.HasPrefix(line, "#EXT-X-PROGRAM-DATE-TIME:"):
			line = line[len("#EXT-X-PROGRAM-DATE-TIME:"):]

			tmp, err := time.Parse("2006-01-02T15:04:05.999Z07:00", line)
			if err != nil {
				return err
			}

			curSegment.DateTime = &tmp

		case line == "#EXT-X-GAP":
			curSegment.Gap = true

		case strings.HasPrefix(line, "#EXT-X-BITRATE:"):
			line = line[len("#EXT-X-BITRATE:"):]

			tmp, err := strconv.ParseInt(line, 10, 64)
			if err != nil {
				return err
			}

			tmp2 := int(tmp)
			curSegment.Bitrate = &tmp2

		case strings.HasPrefix(line, "#EXTINF:"):
			line = line[len("#EXTINF:"):]
			parts := strings.SplitN(line, ",", 2)
			if len(parts) != 2 {
				return fmt.Errorf("invalid EXTINF: %s", line)
			}

			tmp, err := primitives.DurationUnmarshal(parts[0])
			if err != nil {
				return err
			}

			curSegment.Duration = tmp
			curSegment.Title = strings.TrimSpace(parts[1])

		case strings.HasPrefix(line, "#EXT-X-BYTERANGE:"):
			line = line[len("#EXT-X-BYTERANGE:"):]

			tmp1, tmp2, err := primitives.ByteRangeUnmarshal(line)
			if err != nil {
				return err
			}

			curSegment.ByteRangeLength = &tmp1
			curSegment.ByteRangeStart = tmp2

		case strings.HasPrefix(line, "#EXT-X-PART:"):
			line = line[len("#EXT-X-PART:"):]

			var part MediaPart
			err := part.unmarshal(line)
			if err != nil {
				return err
			}

			curSegment.Parts = append(curSegment.Parts, &part)

		case len(line) != 0 && line[0] != '#':
			curSegment.URI = line

			err := curSegment.validate()
			if err != nil {
				return err
			}

			m.Segments = append(m.Segments, curSegment)

			curSegment = &MediaSegment{}

		case strings.HasPrefix(line, "#EXT-X-PRELOAD-HINT:"):
			line = line[len("#EXT-X-PRELOAD-HINT:"):]

			m.PreloadHint = &MediaPreloadHint{}
			err = m.PreloadHint.unmarshal(line)
			if err != nil {
				return err
			}

		case line == "#EXT-X-ENDLIST":
			m.Endlist = true
		}
	}

	m.Parts = curSegment.Parts

	if m.TargetDuration == 0 {
		return fmt.Errorf("TARGETDURATION not set")
	}
	if len(m.Segments) == 0 {
		return fmt.Errorf("no segments found")
	}

	return nil
}

// Marshal encodes the playlist.
func (m Media) Marshal() ([]byte, error) {
	ret := "#EXTM3U\n" +
		"#EXT-X-VERSION:" + strconv.FormatInt(int64(m.Version), 10) + "\n"

	if m.IndependentSegments {
		ret += "#EXT-X-INDEPENDENT-SEGMENTS\n"
	}

	if m.AllowCache != nil {
		var v string
		if *m.AllowCache {
			v = "YES"
		} else {
			v = "NO"
		}
		ret += "#EXT-X-ALLOW-CACHE:" + v + "\n"
	}

	ret += "#EXT-X-TARGETDURATION:" + strconv.FormatInt(int64(m.TargetDuration), 10) + "\n"

	if m.ServerControl != nil {
		ret += m.ServerControl.marshal()
	}

	if m.PartInf != nil {
		ret += m.PartInf.marshal()
	}

	ret += "#EXT-X-MEDIA-SEQUENCE:" + strconv.FormatInt(int64(m.MediaSequence), 10) + "\n"

	if m.DiscontinuitySequence != nil {
		ret += "#EXT-X-DISCONTINUITY-SEQUENCE:" + strconv.FormatInt(int64(m.MediaSequence), 10) + "\n"
	}

	if m.PlaylistType != nil {
		ret += "#EXT-X-PLAYLIST-TYPE=" + string(*m.PlaylistType) + "\n"
	}

	if m.Map != nil {
		ret += m.Map.marshal()
	}

	if m.Skip != nil {
		ret += m.Skip.marshal()
	}

	for _, seg := range m.Segments {
		ret += seg.marshal()
	}

	for _, part := range m.Parts {
		ret += part.marshal()
	}

	if m.PreloadHint != nil {
		ret += m.PreloadHint.marshal()
	}

	if m.Endlist {
		ret += "#EXT-X-ENDLIST\n"
	}

	return []byte(ret), nil
}
