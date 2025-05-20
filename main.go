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
	lbMu 	 sync.Mutex
)

// const (
// 	limit  = 5
// 	window = 10 * time.Second
// )

// var maxToken = 2

func main() {
	r := gin.Default()

	r.Use(IpBasedLim(5, 10*time.Second))

	r.GET("/", func(ctx *gin.Context) {
		ctx.JSON(200, gin.H{
			"message": "working!",
		})
	})

	r.Run(":8080")
}

func IpBasedLim(limit int, window time.Duration ) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		now := time.Now()

	
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

func TokenBucketLim( maxToken int64) gin.HandlerFunc{
	return func(c *gin.Context) {	
		maxMu.Lock()
		if maxToken == 0 {
			maxMu.Unlock()
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "Rate limit exceeded. Try again later.",
			})
			return
		}
		maxToken -= 1
		maxMu.Unlock()
		defer func (){
			maxMu.Lock()
			maxToken++
			maxMu.Unlock()
		}()
		c.Next()
	}
}

func LeakyBucketLim(capacity int, rate int,) gin.HandlerFunc {
	return func(c *gin.Context) {
		lb := struct {
				capacity    int           
				rate        time.Duration 
				mutex       sync.Mutex    
				tokens      int           
				lastLeakage time.Time
		}{capacity: capacity, rate: time.Second / time.Duration(rate), tokens: 0, lastLeakage: time.Now()}

		Allow := func() bool {
			lbMu.Lock()
			defer lbMu.Unlock()

			now := time.Now()
			elapsed := now.Sub(lb.lastLeakage)
			leakedTokens := int(elapsed / lb.rate)

			//update the bucket
			if leakedTokens > 0 {
				if leakedTokens > lb.tokens {
					lb.tokens = 0 
				} else {
					lb.tokens -= leakedTokens
				}
				lb.lastLeakage = now
			}

			// check for adding new tokens
			if lb.tokens < lb.capacity {
				lb.tokens++
			return true
			}

			return false
		}

		if !Allow() {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"status":  429,
				"message": "Too Many Requests",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}