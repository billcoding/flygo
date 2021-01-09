package middleware

import (
	"errors"
	"fmt"
	c "github.com/billcoding/flygo/context"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type uploadFile struct {
	logger  *log.Logger
	root    string
	size    int
	mimes   []string
	exts    []string
	domain  string
	prefix  string
	dateDir bool
}

func UploadFile() *uploadFile {
	return &uploadFile{
		logger:  log.New(os.Stdout, "[uploadFile]", log.LstdFlags),
		root:    os.TempDir(),
		size:    100 * 1024 * 1024,
		mimes:   []string{"text/plain", "image/jpeg", "image/jpg", "image/png", "image/gif", "application/octet-stream"},
		exts:    []string{".txt", ".jpg", ".png", ".gif", ".xlsx"},
		domain:  "http://localhost/",
		prefix:  "/download/downfile",
		dateDir: true,
	}
}

func (uf *uploadFile) Type() *Type {
	return TypeHandler
}

func (uf *uploadFile) Method() Method {
	return MethodPost
}

func (uf *uploadFile) Pattern() Pattern {
	return "/upload/uploadFile"
}

func (uf *uploadFile) Handler() func(c *c.Context) {
	return func(c *c.Context) {

		defer removeTmpFiles(c)
		type jd struct {
			Msg  string `json:"msg"`
			Code int    `json:"code"`
		}
		type jdd struct {
			Msg  string                 `json:"msg"`
			Code int                    `json:"code"`
			Data map[string]interface{} `json:"data"`
		}
		getJson := func(msg string, code int) *jd {
			return &jd{
				Msg:  msg,
				Code: code,
			}
		}
		getJsonData := func(data map[string]interface{}) *jdd {
			return &jdd{
				Msg:  "success",
				Code: 0,
				Data: data,
			}
		}

		err := c.ParseMultipart(int64(uf.size))
		if err != nil {
			uf.logger.Println(err)
			c.JSON(getJson("parse file error", 1))
			return
		}

		files := c.MultipartFiles("file")
		for _, file := range files {
			err := verfiyFile(file, uf)
			if err != nil {
				uf.logger.Println(err)
				c.JSON(getJson(err.Error(), 1))
				return
			}
		}

		ufiles := make([]UFile, 0)
		saveFiles := make([]string, 0)

		dateDir := ""
		if uf.dateDir {
			dateDir = time.Now().Format("20060102")
		}

		parentPath := filepath.Join(uf.root, dateDir)

		_, err = ioutil.ReadDir(parentPath)
		if err != nil {
			err := os.MkdirAll(parentPath, os.ModePerm)
			if err != nil {
				uf.logger.Println(err)
				c.JSON(getJson(err.Error(), 1))
				return
			}
		}

		for _, file := range files {
			extIndex := strings.LastIndexByte(file.Filename, '.')
			ext := file.Filename[extIndex+1:]

			saveFile := fmt.Sprintf("%x.%s", rand.Int63(), ext)
			saveFilePath := filepath.Join(parentPath, saveFile)

			fileUrl := fmt.Sprintf("%s%s?file=%s", uf.domain, uf.prefix, saveFile)
			ufiles = append(ufiles, UFile{
				File: saveFile,
				Url:  fileUrl,
			})

			err := file.Copy(saveFilePath)
			if err != nil {
				uf.logger.Println(err)
				_ = os.Chmod(saveFilePath, os.ModePerm)
				removeSaveFiles(saveFiles)
				c.JSON(getJson(err.Error(), 1))
				return
			}

			saveFiles = append(saveFiles, saveFilePath)
		}

		c.JSON(getJsonData(map[string]interface{}{
			"total": len(files),
			"files": ufiles,
		}))

	}
}

func (uf *uploadFile) Root(root string) *uploadFile {
	uf.root = root
	return uf
}

func (uf *uploadFile) Size(size int) *uploadFile {
	uf.size = size
	return uf
}

func (uf *uploadFile) Exts(exts []string) *uploadFile {
	uf.exts = exts
	return uf
}

func (uf *uploadFile) AddExt(exts ...string) *uploadFile {
	uf.exts = append(uf.exts, exts...)
	return uf
}

func (uf *uploadFile) Mimes(mimes []string) *uploadFile {
	uf.mimes = mimes
	return uf
}

func (uf *uploadFile) AddMime(mimes ...string) *uploadFile {
	uf.mimes = append(uf.mimes, mimes...)
	return uf
}

func (uf *uploadFile) Domain(domain string) *uploadFile {
	uf.domain = domain
	return uf
}

func (uf *uploadFile) Prefix(prefix string) *uploadFile {
	uf.prefix = prefix
	return uf
}

func (uf *uploadFile) DateDir(dateDir bool) *uploadFile {
	uf.dateDir = dateDir
	return uf
}

func (*uploadFile) Name() string {
	return "UploadFile"
}

type UFile struct {
	File string `json:"file"`
	Url  string `json:"url"`
}

func verfiyFile(file *c.MultipartFile, uf *uploadFile) error {

	err := verifySize(file, uf)
	if err != nil {
		return err
	}

	err = verifyMime(file, uf)
	if err != nil {
		return err
	}

	err = verifyExt(file, uf)
	if err != nil {
		return err
	}

	return nil
}

func verifySize(file *c.MultipartFile, uf *uploadFile) error {
	if file.Size > int64(uf.size) {
		return errors.New(fmt.Sprintf("the file size exceed limit[max:%d, current:%d]", uf.size, file.Size))
	}
	return nil
}

func verifyMime(file *c.MultipartFile, uf *uploadFile) error {
	ms := strings.Join(uf.mimes, "|")
	mimeAll := fmt.Sprintf("|%s|", ms)
	if !strings.Contains(mimeAll, fmt.Sprintf("|%s|", file.ContentType)) {
		return errors.New(fmt.Sprintf("the file mime type not supported[supports:%s, current:%s]", ms, file.ContentType))
	}
	return nil
}

func verifyExt(file *c.MultipartFile, uf *uploadFile) error {
	es := strings.Join(uf.exts, "|")
	extAll := fmt.Sprintf("|%s|", es)
	extIndex := strings.LastIndexByte(file.Filename, '.')
	if extIndex == -1 {
		return errors.New(fmt.Sprintf("the file ext not supported[supports:%s, current:%s]", es, ""))
	}
	ext := file.Filename[extIndex:]
	if !strings.Contains(extAll, fmt.Sprintf("|%s|", ext)) {
		return errors.New(fmt.Sprintf("the file ext not supported[supports:%s, current:%s]", es, ext))
	}
	return nil
}

func removeTmpFiles(c *c.Context) {
	defer func() {
		if re := recover(); re != nil {
		}
	}()
	form := c.Request.MultipartForm
	if form != nil {
		_ = form.RemoveAll()
	}
}

func removeSaveFiles(saveFiles []string) {
	for _, f := range saveFiles {
		_ = os.Remove(f)
	}
}
