package main

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/mdaffin/go-telegraf"
	"golang.org/x/image/bmp"
	"golang.org/x/image/tiff"
	"image"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"
	"path/filepath"
	"image/jpeg"
	"net/url"
	"strings"
)

const (
	// DefaultTsDirectoryStructure is the default directory structure for timestreams
	DefaultTsDirectoryStructure = "2006/2006_01/2006_01_02/2006_01_02_15/"
	// TsForm is the timestamp form for individual files.
	TsForm = "2006_01_02_15_04_05"
)

var (
	errLog         *log.Logger
	name           string
	interval       time.Duration
)

type Metric struct {
	OutputSize_b                              int
	DecodeTime_s, EncodeTime_s, RequestTime_s float64
	RequestStatusCode                         int
	RequestContentLength_b                    int64
	RequestStatus                             string
	RequestContentType                        string
}

var netClient = &http.Client{
	Timeout: time.Second * 60,
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

// unmarshalExtraTags turns a list of comma separated key=value pairs into a map
func unmarshalExtraTags(tagString string) (tags map[string]string, err error) {
	for _, pair := range strings.Split(tagString, ","){
		p := strings.Split(pair, "=")
		if len(p) != 2 {
			err = fmt.Errorf("couldn't parse %s into 2 items for extra tags\n", pair)
			return
		}
		tagKey, tagValue := p[0], p[1]
		tags[tagKey] = tagValue
	}
	return
}

func capture(filePath string) {

	telegrafHost := "telegraf:8092"
	if os.Getenv("TELEGRAF_HOST") != ""{
		telegrafHost = os.Getenv("TELEGRAF_HOST")
	}

	telegrafClient, err := telegraf.NewUDP(telegrafHost)
	if err != nil {
		panic(err)

	}
	defer telegrafClient.Close()

	m := telegraf.NewMeasurement("ipcamera")

	urlString := os.Getenv("URL")
	if urlString == "" {
		panic(fmt.Errorf("No URL environment var provided.\n"))
	}

	if extraTags, err := unmarshalExtraTags(os.Getenv("EXTRA_TAGS")); err == nil {
		for tagKey, tagValue := range extraTags {
			m.AddTag(tagKey, tagValue)
		}
	}else{
		errLog.Println(err)
	}

	if u, err := url.Parse(urlString); err == nil{
		m.AddTag("ipaddress", u.Host)
	}else {
		errLog.Println(err)
	}

	st := time.Now()
	resp, err := netClient.Get(urlString)
	m.AddFloat64("RequestTime_s", time.Now().Sub(st).Seconds())
	if err != nil {
		errLog.Println(err)
		return
	}
	m.AddTag("RequestStatus", resp.Status)
	m.AddInt64("RequestStatusCode", int64(resp.StatusCode))

	errLog.Println(resp.Status)
	contentType := resp.Header["Content-Type"][0]
	m.AddTag("Content-Type", contentType)
	var img image.Image
	if contentType == "image/bmp" {
		st = time.Now()
		img, err = bmp.Decode(resp.Body)
		m.AddFloat64("DecodeTime_s", time.Now().Sub(st).Seconds())
		if err != nil {
			errLog.Println(err)
			return
		}
	} else if stringInSlice(contentType, []string{"image/png", "image/jpg", "image/jpeg"}) {
		st = time.Now()
		img, _, err = image.Decode(resp.Body)
		m.AddFloat64("DecodeTime_s", time.Now().Sub(st).Seconds())
		if err != nil {
			errLog.Println(err)
			return
		}
	} else {
		panic(fmt.Errorf("unknown image format %s\n", contentType))
	}
	st = time.Now()
	var imageBytes []byte
	if stringInSlice(os.Getenv("IMAGETYPE"), []string{"jpeg", "jpg", "JPG", "JPEG"}) {
		if stringInSlice(contentType, []string{"image/png", "image/jpg", "image/jpeg"}) {

		}

		var jpegBytes bytes.Buffer
		jpegWriter := bufio.NewWriter(&jpegBytes)
		err = jpeg.Encode(jpegWriter, img, &jpeg.Options{Quality: 90})
		m.AddFloat64("EncodeTime_s", time.Now().Sub(st).Seconds())
		if err != nil {
			panic(err)
		}
		jpegWriter.Flush()
		imageBytes = jpegBytes.Bytes()

	} else {

		var tiffbytes bytes.Buffer
		tiffwriter := bufio.NewWriter(&tiffbytes)
		err = tiff.Encode(tiffwriter, img, &tiff.Options{Compression: tiff.Deflate})
		m.AddFloat64("EncodeTime_s", time.Now().Sub(st).Seconds())
		if err != nil {
			panic(err)
		}

		tiffwriter.Flush()
		imageBytes = tiffbytes.Bytes()

	}
	m.AddFloat64("OutputSize_b", time.Now().Sub(st).Seconds())

	dirPath := filepath.Dir(filePath)
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		os.MkdirAll(dirPath, 0775)
	}

	if err = ioutil.WriteFile(filePath, imageBytes, 0665); err != nil {
		panic(err)
	}
	errLog.Printf("Wrote %s\n", filePath)
	m.AddTag("camera_name", name)

	telegrafClient.Write(m)
	errLog.Printf("Wrote %s\n", m.ToLineProtocal())
}


func init() {
	errLog = log.New(os.Stderr, "[ipeye] ", log.Ldate|log.Ltime|log.Lshortfile)
	intervalStr := os.Getenv("INTERVAL")
	if intervalStr == "" {
		intervalStr = "10m"
	}
	var err error
	interval, err = time.ParseDuration(intervalStr)
	if err != nil {
		panic(err)
	}

	if name = os.Getenv("NAME"); name == "" {
		name, err = os.Hostname()
		if err != nil {
			panic(err)
		}
	}
	errLog.Println(name)
	errLog.Println(interval)
	errLog.Println(os.Getenv("TAGS"))
}

func getImagePath(theTime time.Time) string {
	directory := theTime.Format(DefaultTsDirectoryStructure)
	timestamp := theTime.Format(TsForm)
	filename := fmt.Sprintf("%s_%s_00.tiff", name, timestamp)
	if stringInSlice(os.Getenv("IMAGETYPE"), []string{"jpeg", "jpg", "JPG", "JPEG"}){
		filename = fmt.Sprintf("%s_%s_00.%s", name, timestamp, os.Getenv("IMAGETYPE"))
	}

	outputPath := os.Getenv("OUTPUT")
	if outputPath == ""{
		outputPath = "/data"
	}
	return filepath.Join(outputPath, directory, filename)
}

func main() {
	waitForNextTimepoint := time.After(time.Until(time.Now().Add(interval).Truncate(interval)))
	errLog.Println("Waiting until "+time.Now().Add(interval).Truncate(interval).Format(time.RubyDate))
	select {
	case t:=<-waitForNextTimepoint:
		capture(getImagePath(t.Truncate(interval)))
		break
	}

	ticker := time.NewTicker(interval)
	for {
		select {
		case t:= <-ticker.C:
			capture(getImagePath(t.Truncate(interval)))
			errLog.Println("Waiting until "+time.Now().Add(interval).Truncate(interval).Format(time.RubyDate))
		}
	}
}
