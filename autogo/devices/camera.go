// A Golang API for the Panasonic conference camera

package devices

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const mjpegContentTypePrefix = "multipart/x-mixed-replace; boundary="

type PanasonicIPCam struct {
	ipAddr string
	name string
}

func NewPanasonicIPCam(ipAddr string) (*PanasonicIPCam, error) {
	request, err := http.NewRequest("GET", "http://" + ipAddr + "/cgi-bin/get_basic", nil)
	if err != nil {
		return nil, err
	}
	client := http.Client{
		Timeout: time.Second,
	}
	res, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != 200 {
		return nil, errors.New("Invalid IP address")
	}
	var buf bytes.Buffer
	buf.ReadFrom(res.Body)
	data := buf.String()
	props := strings.Split(data, "\n")
	var title *string
	for i := 0; i < len(props); i++ {
		if strings.HasPrefix(props[i], "cam_title=") {
			t := strings.Trim(props[i][10:], " \r")
			title = &t
			break
		}
	}
	if title == nil {
		return nil, errors.New("Error fetching cam details")
	}

	return &PanasonicIPCam{
		ipAddr: ipAddr,
		name: *title,
	}, nil
}

func (c *PanasonicIPCam) Name() string {
	return c.name
}

func (c *PanasonicIPCam) StreamFrames(count uint) ([][]byte, error) {
	client := http.Client{}
	request, err := http.NewRequest("GET", "http://" + c.ipAddr + "/cgi-bin/mjpeg", nil)
	if err != nil {
		return nil, err
	}
	res, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != 200 {
		return nil, errors.New("Failed to connect to stream")
	}

	contentTypeHeader := res.Header.Get("Content-Type")
	if !strings.HasPrefix(contentTypeHeader, mjpegContentTypePrefix) {
		return nil, errors.New("Camera didn't return a stream")	
	}

	boundary := contentTypeHeader[len(mjpegContentTypePrefix):]

	var buf bytes.Buffer
	for {
		chunk := make([]byte, 1024)
		n, err := res.Body.Read(chunk)
		buf.Write(chunk[0:n])

		boundaryOccurances := bytes.Count(buf.Bytes(), []byte(boundary))

		if boundaryOccurances >= int(count) + 1 {
			res.Body.Close()

			var frames [][]byte
			multipart := bytes.Split(buf.Bytes(), []byte(boundary))[1:count+1]

			for i := 0; i < len(multipart); i++ {
				lines := bytes.Split(multipart[i], []byte("\n"))
				contentLength := -1
				for j := 0; j < len(lines); j++ {
					line := strings.Trim(string(lines[j]), " \r")
					if strings.HasPrefix(line, "Content-length: ") {
						l, err := strconv.Atoi(line[16:])
						if err != nil {
							log.Println(err)
							break
						}
						contentLength = l
						break
					}
				}
				if contentLength == -1 {
					continue
				}
				sep := bytes.Index(multipart[i], []byte("\n\r\n"))
				if sep == -1 || sep + 3 + contentLength >= len(multipart[i]) {
					continue
				}
				jpeg := multipart[i][sep + 3 : sep + 3 + contentLength]
				frames = append(frames, jpeg)
			}

			return frames, nil
		}

		if err == io.EOF {
			break
		}

		if err != nil {
			return nil, errors.New("Failed to read response")
		}
	}

	return [][]byte{}, nil
}

func (c *PanasonicIPCam) sendCommand(cmd string) error {
	res, err := http.Get("http://" + c.ipAddr + "/cgi-bin/aw_ptz?cmd=%23" + cmd + "&res=1")		
	if err != nil {
		return err
	}
	if res.StatusCode != 200 {
		return errors.New("Command rejected by camera")
	}
	return nil
}

type PanTiltAction uint
const (
	PanTiltStop PanTiltAction = iota
	PanTiltUp
	PanTiltDown
	PanTiltLeft
	PanTiltRight
	PanTiltUpLeft
	PanTiltUpRight
	PanTiltDownLeft
	PanTiltDownRight
)

func panTiltActionCode(action PanTiltAction, slow bool) string {
	codes := []string{ "5050", "5075", "5025", "2550", "7550", "2575", "7575", "2525", "7525" }
	slowCodes := []string{ "5050", "5065", "5035", "3550", "6550", "3565", "6565", "3535", "6535" }

	if slow {
		return slowCodes[action]
	} else {
		return codes[action]
	}
}

func (c *PanasonicIPCam) DoPanTilt(action PanTiltAction) error {
	return c.sendCommand("PTS" + panTiltActionCode(action, false))
}

func (c *PanasonicIPCam) PanTiltTimed(action PanTiltAction, duration time.Duration) error {
	c.DoPanTilt(action)
	time.Sleep(duration)
	return c.DoPanTilt(PanTiltStop)
}

func (c *PanasonicIPCam) GoHome() error {
	return c.sendCommand("APC80008000")
}

func (c *PanasonicIPCam) ExecutePreset(id uint) error {
	return c.sendCommand("R" + fmt.Sprintf("%02d", id - 1))	
}

type ZoomAction uint
const (
	ZoomStop ZoomAction = iota
	ZoomIn
	ZoomOut
)

func zoomActionCode(action ZoomAction, slow bool) string {
	if action == ZoomStop {
		return "50"
	}
	if slow {
		if action == ZoomIn {
			return "70"
		} else {
			return "30"
		}
	} else {
		if action == ZoomIn {
			return "80"
		} else {
			return "20"
		}
	}
}

func (c *PanasonicIPCam) DoZoom(action ZoomAction) error {
	return c.sendCommand("Z" + zoomActionCode(action, false))
}

func (c *PanasonicIPCam) ZoomTimed(action ZoomAction, duration time.Duration) error {
	c.DoZoom(action)
	time.Sleep(duration)
	return c.DoZoom(ZoomStop)
}
