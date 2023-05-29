package bitsend

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"strconv"

	"github.com/Mikubill/transfer/apis"
	"github.com/Mikubill/transfer/utils"
)

const download = "https://bitsend.jp/download/%s.html"

func (b bitSend) PostUpload(string, int64) (string, error) {
	if len(b.resp.Files) == 0 {
		return "", fmt.Errorf("upload failed: no file found in response")
	}
	if apis.DebugMode {
		log.Printf("%+v", b.resp)
	}
	fmt.Printf("Download Link: %s\nDelete Link: %s\n",
		fmt.Sprintf(download, b.resp.Files[0].FileKey), b.resp.Files[0].DeleteUrl)
	return fmt.Sprintf(download, b.resp.Files[0].FileKey), nil
}

func (b *bitSend) DoUpload(name string, size int64, file io.Reader) error {

	body, err := newMultipartUpload(uploadConfig{
		fileSize:   size,
		fileName:   name,
		fileReader: file,
		debug:      apis.DebugMode,
	})
	if err != nil {
		return fmt.Errorf("upload returns error: %s", err)
	}

	var respDat uploadResp
	err = json.Unmarshal(body, &respDat)
	if err != nil {
		return fmt.Errorf("json unmarshal returns error: %s", err)
	}
	b.resp = respDat

	return nil
}

func newMultipartUpload(config uploadConfig) ([]byte, error) {
	if config.debug {
		log.Printf("start upload")
	}
	client := http.Client{}

	byteBuf := &bytes.Buffer{}
	writer := multipart.NewWriter(byteBuf)
	_ = writer.WriteField("u_key", utils.GenRandString(16))
	_, err := writer.CreateFormFile("files[]", config.fileName)
	if err != nil {
		return nil, err
	}

	writerLength := byteBuf.Len()
	writerBody := make([]byte, writerLength)
	_, _ = byteBuf.Read(writerBody)
	_ = writer.Close()

	boundary := byteBuf.Len()
	lastBoundary := make([]byte, boundary)
	_, _ = byteBuf.Read(lastBoundary)

	totalSize := int64(writerLength) + config.fileSize + int64(boundary)
	partR, partW := io.Pipe()

	go func() {
		_, _ = partW.Write(writerBody)
		for {
			buf := make([]byte, 256)
			nr, err := io.ReadFull(config.fileReader, buf)
			if nr <= 0 {
				break
			}
			if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
				fmt.Println(err)
				break
			}
			if nr > 0 {
				_, _ = partW.Write(buf[:nr])
			}
		}
		_, _ = partW.Write(lastBoundary)
		_ = partW.Close()
	}()

	req, err := http.NewRequest("POST", "https://bitsend.jp/jqu/", partR)
	if err != nil {
		return nil, err
	}
	req.ContentLength = totalSize
	req.Header.Set("referer", "https://bitsend.jp/")
	req.Header.Set("content-length", strconv.FormatInt(totalSize, 10))
	req.Header.Set("content-type", fmt.Sprintf("multipart/form-data; boundary=%s", writer.Boundary()))
	if config.debug {
		log.Printf("header: %v", req.Header)
	}
	resp, err := client.Do(req)
	if err != nil {
		if config.debug {
			log.Printf("do requests returns error: %v", err)
		}
		return nil, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		if config.debug {
			log.Printf("read response returns: %v", err)
		}
		return nil, err
	}
	_ = resp.Body.Close()
	if config.debug {
		log.Printf("returns: %v", string(body))
	}

	return body, nil
}