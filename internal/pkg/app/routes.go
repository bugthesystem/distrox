package app

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ziyasal/distroxy/pkg/distrox"
)

const (
	apiVersion  = "v1"
	apiBasePath = "/" + apiVersion + "/"

	// path to cache.
	cachePath  = apiBasePath + "kv"
	statsPath  = apiBasePath + "stats"
	healthPath = "/health"
)

func (s *Server) newRouter() *gin.Engine {
	r := gin.Default()
	r.PUT(cachePath+"/:key", s.putHandler)
	r.GET(cachePath+"/:key", s.getHandler)
	r.DELETE(cachePath+"/:key", s.deleteHandler)

	// exposes cache stats, this could be exported as prometheus metrics
	r.GET(statsPath, s.statsHandler)
	r.GET(healthPath, s.healthHandler)

	return r
}

func (s *Server) putHandler(ctx *gin.Context) {
	key := ctx.Param("key")
	if ok, msg := validateKey(fmt.Sprintf("%s", key), s.cache.MaxKeySizeInBytes); !ok {
		s.logger.Debug(fmt.Sprintf("%s - op: %s", msg, ctx.Request.Method))
		ctx.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}

	valueBytes, err := ioutil.ReadAll(ctx.Request.Body)
	if err != nil {
		s.logger.Err("An error occurred while value bytes from request: ", err)
		ctx.JSON(http.StatusInternalServerError,
			gin.H{"error": "An error occurred while value bytes from request"})
		return
	}

	if ok, msg := validateValue(valueBytes, s.cache.MaxValueSizeInBytes); !ok {
		s.logger.Debug(msg)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}

	keyBytes := []byte(fmt.Sprintf("%s", key))
	if err := s.cache.SetBin(keyBytes, valueBytes); err != nil {
		msg := "An error occurred while storing valueBytes to cache"
		s.logger.Err(msg, err)
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": msg})
		return
	}

	s.logger.Printf("stored '%s' in cache.", key)

	// return empty body with location header
	ctx.Status(http.StatusCreated)
	ctx.Header("Location", fmt.Sprintf("%s/%s", cachePath, key))
}

func (s *Server) getHandler(ctx *gin.Context) {
	key := ctx.Param("key")

	if ok, msg := validateKey(fmt.Sprintf("%s", key), s.cache.MaxKeySizeInBytes); !ok {
		s.logger.Debug(fmt.Sprintf("%s - op: %s", msg, ctx.Request.Method))
		ctx.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}

	keyBytes := []byte(fmt.Sprintf("%s", key))
	entry, err := s.cache.GetBin(nil, keyBytes)
	if err != nil {
		s.handleError(ctx, err)
		return
	}

	_, err = ctx.Writer.Write(entry)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError,
			gin.H{"error": "value bytes could not written to response"})
	}
}

func (s *Server) deleteHandler(ctx *gin.Context) {
	key := ctx.Param("key")

	if ok, msg := validateKey(fmt.Sprintf("%s", key), s.cache.MaxKeySizeInBytes); !ok {
		s.logger.Debug(fmt.Sprintf("%s - op: %s", msg, ctx.Request.Method))
		ctx.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}

	err := s.cache.Del(fmt.Sprintf("%s", key))

	if err != nil {
		s.handleError(ctx, err)
		return
	}

	ctx.Status(http.StatusOK)
}

func (s *Server) statsHandler(ctx *gin.Context) {
	var stats distrox.CacheStats
	s.cache.LoadStats(&stats)

	ctx.JSON(http.StatusOK, stats)
}

func (s *Server) healthHandler(ctx *gin.Context) {
	// more health indicators could be used here apart from ping
	ctx.Status(http.StatusOK)
}

func (s *Server) handleError(ctx *gin.Context, err error) {
	if errors.Is(err, distrox.ErrEntryNotFound) {
		s.logger.Err(fmt.Sprintf("entry not found on the cache — %s", ctx.Request.Method), err)
		ctx.Status(http.StatusNotFound)
		return
	}

	s.logger.Err(fmt.Sprintf("an error occurred while performing %s", ctx.Request.Method), err)
	ctx.Status(http.StatusInternalServerError)
}
