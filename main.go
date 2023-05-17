package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"log/syslog"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	uuid "github.com/satori/go.uuid"
)

type ResultOCR struct {
	Filename string `json:"filename"`
	Data     string `json:"data,omitempty"`
	Error    string `json:"error,omitempty"`
}

type UploadFile struct {
	Name string
	Src  string
}

var MaxSizeUploadFile int = 5 * 1024 * 1024
var MaxSizeRequest int = 50 * 1024 * 1024

func OCRPost(c *fiber.Ctx) error {
	var err error

	reLang := regexp.MustCompile(`^(rus|eng)$`)
	reMime := regexp.MustCompile(`(application/pdf|image/(jpeg|png|tiff))`)
	result := []ResultOCR{}
	buf := make([]byte, 16384)
	uploadedFiles := []UploadFile{}

	req := c.Request()
	form, err := req.MultipartForm()
	if err != nil {
		return err
	}

	if !req.IsBodyStream() {
		return fmt.Errorf("not a body stream")
	}

	allFilesLen := 0

	for _, files := range form.File {
		fileLen := 0
		for _, file := range files {
			currentResult := ResultOCR{
				Filename: file.Filename,
			}

			if !reMime.MatchString(file.Header["Content-Type"][0]) {
				currentResult.Error = "Unsupported file type"
				result = append(result, currentResult)
				continue
			}

			fin, err := file.Open()
			if err != nil {
				currentResult.Error = err.Error()
				result = append(result, currentResult)
				continue
			}

			srcFile := path.Join(os.TempDir(), uuid.NewV4().String()+strings.ToLower(path.Ext(file.Filename)))

			fout, err := os.Create(srcFile)
			if err != nil {
				fin.Close()
				currentResult.Error = err.Error()
				result = append(result, currentResult)
				continue
			}

			readErrStr := ""

			for {
				nRead, err := fin.Read(buf)
				if err != nil && err != io.EOF {
					readErrStr = err.Error()
					break
				}

				fileLen = fileLen + nRead

				if fileLen > MaxSizeUploadFile {
					readErrStr = "File is too big"
					break
				}

				if nRead == 0 {
					break
				}

				if _, err = fout.Write(buf[:nRead]); err != nil {
					readErrStr = err.Error()
					break
				}
			}

			fin.Close()
			fout.Close()

			if readErrStr != "" {
				os.Remove(srcFile)
				currentResult.Error = readErrStr
				result = append(result, currentResult)
				continue
			}

			if fileLen == 0 {
				currentResult.Error = "Empty file"
				result = append(result, currentResult)
				continue
			}

			uploadedFiles = append(uploadedFiles, UploadFile{
				Name: file.Filename,
				Src:  srcFile,
			})

			allFilesLen = allFilesLen + fileLen

			if allFilesLen > MaxSizeRequest {
				for _, uFile := range uploadedFiles {
					os.Remove(uFile.Src)
				}
				return c.SendStatus(fiber.StatusRequestEntityTooLarge)
			}
		}
	}

	lang := ""

	for _, lv := range form.Value["lang"] {
		slv := strings.Split(strings.ToLower(lv), ",")
		for _, llv := range slv {
			if reLang.MatchString(llv) {
				if lang != "" {
					lang = lang + "+"
				}
				lang = lang + llv
			}
		}
	}

	if lang == "" {
		lang = "rus"
	}

	for _, uFile := range uploadedFiles {
		currentResult := ResultOCR{
			Filename: uFile.Name,
		}

		cmdtext := ""

		if path.Ext(uFile.Src) == ".pdf" {
			cmdtext = fmt.Sprintf("ocrmypdf --language %s --quiet --force-ocr --optimize 0 --rotate-pages --deskew \"%s\" - | pdftotext -layout - -", lang, uFile.Src)
		} else {
			cmdtext = fmt.Sprintf("convert \"%s\" -colorspace Gray - | img2pdf - | ocrmypdf --language %s --quiet --force-ocr --optimize 0 --rotate-pages --deskew - - | pdftotext -layout - -", uFile.Src, lang)
		}

		cmd := exec.Command("/bin/sh", "-c", cmdtext)
		out, err := cmd.CombinedOutput()
		os.Remove(uFile.Src)
		if err != nil {
			currentResult.Error = string(out)
			result = append(result, currentResult)
			continue
		}

		currentResult.Data = string(out)
		result = append(result, currentResult)

	}

	return c.JSON(result)
}

func main() {
	var daemonize bool
	var err error
	var ui64 uint64

	EM1 := "Application \"%s\" not found\n"
	// EM2 := "Environment variable %s required\n"

	flag.BoolVar(&daemonize, "d", false, "application was started as a daemon")
	flag.Parse()

	if daemonize {
		syslogger, err := syslog.New(syslog.LOG_INFO, "WEBOCRD")
		if err != nil {
			log.Fatalln(err)
		}

		log.SetOutput(syslogger)
	}

	cmd := exec.Command("/bin/sh", "-c", "command -v ocrmypdf")
	if err = cmd.Run(); err != nil {
		log.Fatalf(EM1, "ocrmypdf")
	}

	cmd = exec.Command("/bin/sh", "-c", "command -v pdftotext")
	if err = cmd.Run(); err != nil {
		log.Fatalf(EM1, "pdftotext")
	}

	cmd = exec.Command("/bin/sh", "-c", "command -v convert")
	if err = cmd.Run(); err != nil {
		log.Fatalf(EM1, "convert")
	}

	cmd = exec.Command("/bin/sh", "-c", "command -v img2pdf")
	if err = cmd.Run(); err != nil {
		log.Fatalf(EM1, "img2pdf")
	}

	httpAddr := strings.TrimSpace(os.Getenv("WEBOCRD_HTTP_ADDR"))
	if httpAddr == "" {
		// log.Fatalf(EM2, "WEBOCRD_HTTP_ADDR")
		httpAddr = "127.0.0.1:3000"
	}

	fileSizeStr := strings.TrimSpace(os.Getenv("WEBOCRD_MAX_FILE_SIZE"))
	if fileSizeStr != "" {
		ui64, err = strconv.ParseUint(fileSizeStr, 10, 32)
		if err == nil {
			MaxSizeUploadFile = int(ui64)
		}
	}

	requestSizeStr := strings.TrimSpace(os.Getenv("WEBOCRD_MAX_REQ_SIZE"))
	if requestSizeStr != "" {
		ui64, err = strconv.ParseUint(requestSizeStr, 10, 32)
		if err == nil {
			MaxSizeRequest = int(ui64)
		}
	}

	app := fiber.New(fiber.Config{
		DisablePreParseMultipartForm: true,
		StreamRequestBody:            true,
	})

	app.Use(cors.New())
	app.Static("/", "./dist")

	v1 := app.Group("/api/v1")
	v1.Post("/ocr", OCRPost)

	app.Listen(httpAddr)
}
