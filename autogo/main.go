package main

import (
	"autogo/devices"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"log"
	"os"
	"sync"

	"gocv.io/x/gocv"
)

type jsonCameraInfo struct {
	IP string	`json:"ip"`
	Name string `json:"d"`
}

func main() {
	jsonBytes, err := os.ReadFile("cameras.json")	
	if err != nil {
		log.Fatalln(err)
	}

	var cameras []jsonCameraInfo
	err = json.Unmarshal(jsonBytes, &cameras)
	if err != nil {
		log.Fatalln(err)
	}

	var wg sync.WaitGroup
	wg.Add(len(cameras))

	for i := 0; i < len(cameras); i++ {
		go func(i int) {
			defer wg.Done()

			camera, err := devices.NewPanasonicIPCam(cameras[i].IP)
			if err != nil {
				fmt.Printf("%s: no response\n", cameras[i].Name)
				return
			}

			checkCameraForMotion(camera)
		}(i)
	}

	wg.Wait()
}

func checkCameraForMotion(camera *devices.PanasonicIPCam) {
	frames, err := camera.StreamFrames(10)
	if err != nil {
		log.Println(err)
		return
	}

	imgDelta := gocv.NewMat()
	imgThresh := gocv.NewMat()
	mog2 := gocv.NewBackgroundSubtractorMOG2()

	motionDetected := false

	for i := 0; i < len(frames); i++ {
		img, err := gocv.IMDecode(frames[i], gocv.IMReadUnchanged)
		if err != nil {
			log.Println(err)
			return
		}

		mog2.Apply(img, &imgDelta)
		gocv.Threshold(imgDelta, &imgThresh, 25, 255, gocv.ThresholdBinary)
		
		kernel := gocv.GetStructuringElement(gocv.MorphRect, image.Pt(3, 3))
		gocv.Dilate(imgThresh, &imgThresh, kernel)
		kernel.Close()

		contours := gocv.FindContours(imgThresh, gocv.RetrievalExternal, gocv.ChainApproxSimple)

		for j := 0; j < contours.Size() && i != 0; j++ {
			area := gocv.ContourArea(contours.At(j))
			if area < 1000 {
				continue
			}

			motionDetected = true

			gocv.DrawContours(&img, contours, j, color.RGBA{255, 0, 0, 0}, 2)
			gocv.Rectangle(&img, gocv.BoundingRect(contours.At(j)), color.RGBA{0, 255, 0, 0}, 2)
		}

		if motionDetected {
			gocv.IMWrite(fmt.Sprintf("out/%s-%d.jpeg", camera.Name(), i), img)
		}

		contours.Close()
		img.Close()
	}

	if motionDetected {
		fmt.Printf("%s: occupied\n", camera.Name())
	} else {
		fmt.Printf("%s: unoccupied \n", camera.Name())
	}
}
