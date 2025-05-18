package main

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

var (
	requests = make(map[string][]time.Time)
	maxMu       sync.Mutex
	reqMu       sync.Mutex

)

const (
	limit  = 5
	window = 10 * time.Second
)

var max = 2

func main() {
	r := gin.Default()

	r.Use(RateLimiterMiddleware())

	r.GET("/", func(ctx *gin.Context) {
		ctx.JSON(200, gin.H{
			"message": "working!",
		})
	})

	r.Run(":8080")
}

func RateLimiterMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		now := time.Now()

		maxMu.Lock()
		if max == 0 {
			maxMu.Unlock()
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "maxxxxxxxxx.",
			})
			return
		}
		max -= 1
		maxMu.Unlock()
		defer func (){
			maxMu.Lock()
			max++
			maxMu.Unlock()
		}()

		reqMu.Lock()
		//clean old timestamps
		timestamps := requests[ip]
		var newTimestamps []time.Time
		for _, t := range timestamps {
			if now.Sub(t) < window {
				newTimestamps = append(newTimestamps, t)
			}
		}

		//if limit exceeded
		if len(newTimestamps) >= limit {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "Rate limit exceeded. Try again later.",
			})
			return
		}

		// add current timestamp
		newTimestamps = append(newTimestamps, now)
		requests[ip] = newTimestamps
		reqMu.Unlock()

		c.Next()
	}
}


