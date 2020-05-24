package main

import (
	"fmt"
	"net/http"
	"os"
	"sync"

	"github.com/gin-gonic/gin"
)

type ShareFile struct {
	Secret      string
	Port        *int
	code        map[string]*os.File
	server      *gin.Engine
	map_mutex   sync.Mutex
	working_map map[string]bool
}

func CreateShareObject() *ShareFile {
	var share ShareFile
	share.code = make(map[string]*os.File)
	share.working_map = make(map[string]bool)
	if share.Port == nil {
		share.Port = new(int)
		*share.Port = 8800
	}
	return &share
}

func (self *ShareFile) Share(code string, file *os.File) error {
	self.code = make(map[string]*os.File)
	_, err := file.Seek(0, 0)
	if err != nil {
		return err
	}
	_, err = file.Stat()
	if err != nil {
		return err
	}
	self.code[code] = file
	return nil
}

func (self *ShareFile) Start() {
	if self.server != nil {
		return
	}
	self.server = gin.Default()
	self.server.GET("/download", func(c *gin.Context) {
		if c.Request.URL.Query().Get("secret") != self.Secret {
			c.Status(http.StatusNotFound)
			return
		}
		code := c.Request.URL.Query().Get("code")
		_, ok := self.working_map[code]
		if ok {
			c.Status(http.StatusConflict)
			return
		}
		self.map_mutex.Lock()
		_, ok = self.working_map[code]
		if ok {
			c.Status(http.StatusConflict)
			self.map_mutex.Unlock()
			return
		}
		file, ok := self.code[code]
		if !ok {
			c.Status(http.StatusNotFound)
			self.map_mutex.Unlock()
			return
		}
		self.working_map[code] = true
		_, err := file.Seek(0, 0)
		if err != nil {
			c.Status(http.StatusBadGateway)
			delete(self.code, code)
			delete(self.working_map, code)
			return
		}
		self.map_mutex.Unlock()
		info, err := file.Stat()
		if err != nil {
			c.Status(http.StatusBadGateway)
			delete(self.code, code)
			delete(self.working_map, code)
			return
		}
		filename := file.Name()
		defer delete(self.working_map, code)
		c.DataFromReader(http.StatusOK, info.Size(), "application/file", file, map[string]string{
			"Content-Disposition": fmt.Sprintf(`attachment; filename="%s"`, filename),
		})
	})
	self.server.NoRoute(func(c *gin.Context) {
		c.Status(http.StatusNotFound)
	})
	self.server.Run(fmt.Sprintf("0.0.0.0:%d", *self.Port))
}
