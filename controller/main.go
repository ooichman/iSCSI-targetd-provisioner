package main

import (
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

type Config struct {
	PythonAPI string
	TargetIQN string
	Portals   []string
	Multipath bool
}

func main() {
	cfg := Config{
		PythonAPI: os.Getenv("ISCSI_API_ENDPOINT"),
		TargetIQN: os.Getenv("ISCSI_TARGET_IQN"),
		Portals:   strings.Split(os.Getenv("ISCSI_PORTALS"), ","),
		Multipath: os.Getenv("ISCSI_MULTIPATH_ENABLED") == "true",
	}

	r := gin.Default()

	// --- PRODUCTION ROUTES ---
	v1 := r.Group("/v1")
	{
		// Get volume info (Used by Node Plugin)
		v1.GET("/volumes/:pvcName", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"iqn":               cfg.TargetIQN,
				"portals":           cfg.Portals,
				"multipath_enabled": cfg.Multipath,
				"lun":               0,
			})
		})

		// Real Provisioning (Calls Python API)
		v1.POST("/volumes/provision", func(c *gin.Context) {
			// Logic to call callPythonAPI() goes here
			c.JSON(http.StatusOK, gin.H{"status": "provisioning_started"})
		})
	}

	// --- DEMO / TESTING ROUTES ---
	demo := r.Group("/demo")
	{
		demo.POST("/provision", func(c *gin.Context) {
			var params map[string]interface{}
			c.BindJSON(&params)
			
			log.Printf("[DEMO MODE] Intercepted request for: %v", params["name"])
			log.Printf("[DEMO MODE] Would have called API: %s", cfg.PythonAPI)
			
			c.JSON(http.StatusOK, gin.H{
				"status": "success",
				"mode":   "mock_demo",
				"message": "No real volume was created on the storage server",
			})
		})
	}

	log.Println("iSCSI Controller active on :8080")
	r.Run(":8080")
}
