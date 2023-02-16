package servercontroll

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"gitee.com/dark.H/gs"
)

func uploadFileFunc(www string) (o func(w http.ResponseWriter, r *http.Request)) {

	o = func(w http.ResponseWriter, r *http.Request) {
		gs.Str("File Upload Endpoint Hit").Println()

		// Parse our multipart form, 10 << 20 specifies a maximum
		// upload of 10 MB files.
		r.ParseMultipartForm(10 << 30)
		// FormFile returns the first file for the given key `myFile`
		// it also returns the FileHeader so we can get the Filename,
		// the Header and the size of the file
		file, handler, err := r.FormFile("myFile")
		// fmt.Println(err)
		handler.Filename = string(gs.Str(handler.Filename).Printable(true))
		if err != nil {
			gs.Str("Error Retrieving the File").Println()
			gs.Str(err.Error()).Print("err")
			return
		}
		defer file.Close()
		if gs.Str(handler.Filename).In("..") {
			w.WriteHeader(501)
			w.Write([]byte("Fu"))
			return
		}
		if gs.Str(handler.Filename).In("/") {
			w.WriteHeader(501)
			w.Write([]byte("Fu"))
			return
		}

		if gs.Str(handler.Filename).In(" ") {
			handler.Filename = string(gs.Str(handler.Filename).Replace(" ", "_"))
			// return
		}

		gs.Str("Uploaded File: %+v\n").F(handler.Filename).Print()
		gs.Str("File Size: %+v\n").F(handler.Size).Print()
		gs.Str("MIME Header: %+v\n").F(handler.Header).Print()

		// Create a temporary file within our temp-images directory that follows
		// a particular naming pattern
		tmpdir := os.TempDir()
		tempFile, err := ioutil.TempFile(tmpdir, "upload-*.file")
		if err != nil {
			gs.Str(err.Error()).Println("err")
		}
		defer tempFile.Close()

		// read all of the contents of our uploaded file into a
		// byte array
		fileBytes, err := ioutil.ReadAll(file)
		if err != nil {
			gs.Str(err.Error()).Println("err")
		}
		// write this byte array to our temporary file
		tempFile.Write(fileBytes)
		tempFile.Close()
		realFilePath := gs.Str(www).PathJoin(handler.Filename)
		if err := os.Rename(tempFile.Name(), realFilePath.Str()); err != nil {
			w.WriteHeader(504)
			fmt.Fprintf(w, err.Error())
			return
		}
		// return that we have successfully uploaded our file!
		fmt.Fprintf(w, "Successfully Uploaded File\n")

	}
	return o
}
