package servercontroll

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"time"

	"gitee.com/dark.H/ProxyZ/asset"
	"gitee.com/dark.H/gs"
	"github.com/lucas-clemente/quic-go"
	"github.com/lucas-clemente/quic-go/http3"
)

func GetHTTP3Client(usetls bool, timeout ...int) (client *http.Client) {
	var qconf quic.Config
	cerPEM, err := asset.Asset(CERT)
	if err != nil {
		log.Fatal(err)
	}
	keyPEM, err := asset.Asset(KEYPEM)
	if err != nil {
		log.Fatal(err)
	}

	// Load the certificate and private key
	cert, err := tls.X509KeyPair(cerPEM, keyPEM)
	if err != nil {
		panic(err)
	}
	certpool := x509.NewCertPool()
	certpool.AppendCertsFromPEM(cerPEM)

	config := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		RootCAs:            certpool,
		ClientCAs:          certpool,
		InsecureSkipVerify: true,
	}
	t := 8
	if timeout != nil {
		t = timeout[0]
	}
	if usetls {

		tr := &http.Transport{TLSClientConfig: config}
		client := &http.Client{
			Transport: tr,
			Timeout:   time.Duration(t) * time.Second,
		}
		return client
	}

	roundTripper := &http3.RoundTripper{
		TLSClientConfig: config,
		QuicConfig:      &qconf,
	}
	defer roundTripper.Close()
	hclient := &http.Client{
		Transport: roundTripper,
		Timeout:   time.Duration(t) * time.Second,
	}

	return hclient
}

func HTTP3(addr string, usetls bool, with func(addr string, client *http.Client) (resp *http.Response, err error), timeout ...int) (reply gs.Str, nerr error) {
	cli := GetHTTP3Client(usetls, timeout...)
	if cli != nil {
		if resp, err := with(addr, cli); err != nil {
			reply = gs.Dict[any]{
				"status": "fail",
				"msg":    "req err:" + err.Error(),
			}.Json()
			nerr = err
			return
		} else {
			defer resp.Body.Close()
			buf, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				reply = gs.Dict[any]{
					"status": "fail",
					"msg":    "res err:" + err.Error(),
				}.Json()
				return
			} else {
				b := gs.Str(buf)
				if b.StartsWith("{") && b.EndsWith("}") {
					reply = b
					return
				} else {
					reply = gs.Dict[any]{
						"status": "ok",
						"msg":    b,
					}.Json()
					return
				}
			}
		}
	}
	return
}

func HTTPSGet(addr string) (reply gs.Str, nerr error) {
	reply, nerr = HTTP3(addr, true, func(addr string, client *http.Client) (resp *http.Response, err error) {
		resp, err = client.Get(addr)
		return
	})
	return
}

func HTTP3Get(addr string) (reply gs.Str, nerr error) {
	reply, nerr = HTTP3(addr, false, func(addr string, client *http.Client) (resp *http.Response, err error) {
		resp, err = client.Get(addr)
		return
	})
	return
}

func HTTPSPost(addr string, data gs.Dict[any], timeout ...int) (reply gs.Str, nerr error) {
	reply, nerr = HTTP3(addr, true, func(addr string, client *http.Client) (resp *http.Response, err error) {
		buffer := bytes.NewBufferString(data.Json().Str())
		resp, err = client.Post(addr, "application/json", buffer)
		return
	}, timeout...)
	return
}

func HTTP3Post(addr string, data gs.Dict[any]) (reply gs.Str, nerr error) {
	reply, nerr = HTTP3(addr, false, func(addr string, client *http.Client) (resp *http.Response, err error) {
		buffer := bytes.NewBufferString(data.Json().Str())
		resp, err = client.Post(addr, "application/json", buffer)
		return
	})
	return
}

func HTTP3UploadFile(addr, filePath gs.Str) (reply gs.Str, nerr error) {
	if !addr.EndsWith("/z-files-u") {
		addr += "/z-files-u"
	}
	reply, nerr = HTTP3(addr.Str(), false, func(addr string, client *http.Client) (resp *http.Response, err error) {
		if filePath.IsExists() && !filePath.IsDir() {
			file, err := os.OpenFile(filePath.Str(), os.O_RDONLY, os.ModePerm)
			if err != nil {
				return nil, err
			}
			buffer := &bytes.Buffer{}
			writer := multipart.NewWriter(buffer)
			part, err := writer.CreateFormFile("myFile", file.Name())
			if err != nil {
				return nil, err
			}
			_, err = io.Copy(part, file)
			if err != nil && err != io.EOF {
				return nil, err
			}
			writer.Close()
			req, err := http.NewRequest("POST", addr, buffer)
			if err != nil && err != io.EOF {
				return nil, err
			}
			req.Header.Set("Content-Type", writer.FormDataContentType())
			resp, err = client.Do(req)
			// resp, err = client.Post(addr, writer.FormDataContentType(), R)
			return resp, err
		} else {
			return nil, errors.New("file not exists : " + filePath.Str())
		}
	})
	return
}

func HTTPSUploadFile(addr, filePath gs.Str) (reply gs.Str, nerr error) {
	if !addr.EndsWith("/z-files-u") {
		addr += "/z-files-u"
	}
	reply, nerr = HTTP3(addr.Str(), true, func(addr string, client *http.Client) (resp *http.Response, err error) {
		if filePath.IsExists() && !filePath.IsDir() {
			file, err := os.OpenFile(filePath.Str(), os.O_RDONLY, os.ModePerm)
			if err != nil {
				return nil, err
			}
			buffer := &bytes.Buffer{}
			writer := multipart.NewWriter(buffer)
			part, err := writer.CreateFormFile("myFile", file.Name())
			if err != nil {
				return nil, err
			}
			_, err = io.Copy(part, file)
			if err != nil && err != io.EOF {
				return nil, err
			}
			writer.Close()
			req, err := http.NewRequest("POST", addr, buffer)
			if err != nil && err != io.EOF {
				return nil, err
			}
			req.Header.Set("Content-Type", writer.FormDataContentType())
			resp, err = client.Do(req)
			// resp, err = client.Post(addr, writer.FormDataContentType(), R)
			return resp, err
		} else {
			return nil, errors.New("file not exists : " + filePath.Str())
		}
	})
	return
}

func HTTP3DownFile(addr, fileName, filePath gs.Str) (reply gs.Str, nerr error) {
	if !addr.In("/z-files-d") {
		addr += "/z-files-d/" + fileName
	}
	reply, nerr = HTTP3(addr.Str(), false, func(addr string, client *http.Client) (resp *http.Response, err error) {

		file, err := os.OpenFile(filePath.Str(), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.ModePerm)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		resp, err = client.Get(addr)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		_, err = io.Copy(file, resp.Body)
		if err != nil && err != io.EOF {
			return nil, err
		}
		return resp, err

	})
	return
}

func HTTPSDownFile(addr, fileName, filePath gs.Str) (reply gs.Str, nerr error) {
	if !addr.In("/z-files-d") {
		addr += "/z-files-d/" + fileName
	}
	reply, nerr = HTTP3(addr.Str(), true, func(addr string, client *http.Client) (resp *http.Response, err error) {

		file, err := os.OpenFile(filePath.Str(), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.ModePerm)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		resp, err = client.Get(addr)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		_, err = io.Copy(file, resp.Body)
		if err != nil && err != io.EOF {
			return nil, err
		}
		return resp, err

	})
	return
}
