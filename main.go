package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Models
type AppVersion struct {
	AppName        string    `json:"appName"`
	IOSVersion     string    `json:"iosVersion"`
	AndroidVersion string    `json:"androidVersion"`
	AppStoreURL    string    `json:"appstoreUrl"`
	PlayStoreURL   string    `json:"playstoreUrl"`
	IsRelease      bool      `json:"isRelease"`
	IsActive       bool      `json:"isActive"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type Database struct {
	Apps []AppVersion `json:"apps"`
	mu   sync.RWMutex
}

type Response struct {
	Status  string      `json:"status"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

var db *Database

const dataFile = "database.json"

// Database methods
func (d *Database) Load() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	data, err := os.ReadFile(dataFile)
	if err != nil {
		if os.IsNotExist(err) {
			d.Apps = []AppVersion{}
			return d.Save()
		}
		return err
	}
	return json.Unmarshal(data, &d.Apps)
}

func (d *Database) Save() error {
	data, err := json.MarshalIndent(d.Apps, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(dataFile, data, 0644)
}

func (d *Database) GetAppByName(appName string) (*AppVersion, int) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	for i, app := range d.Apps {
		if app.AppName == appName && app.IsActive {
			return &app, i
		}
	}
	return nil, -1
}

func (d *Database) CreateApp(app AppVersion) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	for _, a := range d.Apps {
		if a.AppName == app.AppName && a.IsActive {
			return gin.Error{Err: http.ErrAbortHandler, Meta: "App already exists"}
		}
	}

	app.IsActive = true
	app.CreatedAt = time.Now()
	app.UpdatedAt = time.Now()
	d.Apps = append(d.Apps, app)
	return d.Save()
}

func (d *Database) UpdateApp(appName string, updated AppVersion) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	for i, app := range d.Apps {
		if app.AppName == appName && app.IsActive {
			d.Apps[i].IOSVersion = updated.IOSVersion
			d.Apps[i].AndroidVersion = updated.AndroidVersion
			d.Apps[i].AppStoreURL = updated.AppStoreURL
			d.Apps[i].PlayStoreURL = updated.PlayStoreURL
			d.Apps[i].IsRelease = updated.IsRelease
			d.Apps[i].UpdatedAt = time.Now()
			return d.Save()
		}
	}
	return gin.Error{Err: http.ErrAbortHandler, Meta: "App not found"}
}

func (d *Database) DeleteApp(appName string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	for i, app := range d.Apps {
		if app.AppName == appName && app.IsActive {
			d.Apps[i].IsActive = false
			d.Apps[i].UpdatedAt = time.Now()
			return d.Save()
		}
	}
	return gin.Error{Err: http.ErrAbortHandler, Meta: "App not found"}
}

func (d *Database) GetAllApps() []AppVersion {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var activeApps []AppVersion
	for _, app := range d.Apps {
		if app.IsActive {
			activeApps = append(activeApps, app)
		}
	}
	return activeApps
}

// Handlers
func healthHandler(c *gin.Context) {
	appName := c.DefaultQuery("appName", "mone_task_app")

	app, _ := db.GetAppByName(appName)
	if app == nil {
		c.JSON(http.StatusNotFound, Response{
			Status:  "error",
			Message: "App not found",
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Status:  "success",
		Message: "App version retrieved",
		Data: gin.H{
			"iosVersion":     app.IOSVersion,
			"androidVersion": app.AndroidVersion,
			"appstoreUrl":    app.AppStoreURL,
			"playstoreUrl":   app.PlayStoreURL,
			"isRelease":      app.IsRelease,
		},
	})
}

func createAppHandler(c *gin.Context) {
	var app AppVersion
	if err := c.ShouldBindJSON(&app); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Status:  "error",
			Message: err.Error(),
		})
		return
	}

	if err := db.CreateApp(app); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Status:  "error",
			Message: "App already exists",
		})
		return
	}

	c.JSON(http.StatusCreated, Response{
		Status:  "success",
		Message: "App created successfully",
		Data:    app,
	})
}

func getAllAppsHandler(c *gin.Context) {
	apps := db.GetAllApps()
	c.JSON(http.StatusOK, Response{
		Status:  "success",
		Message: "Apps retrieved successfully",
		Data:    apps,
	})
}

func updateAppHandler(c *gin.Context) {
	appName := c.Param("appName")

	var updated AppVersion
	if err := c.ShouldBindJSON(&updated); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Status:  "error",
			Message: err.Error(),
		})
		return
	}

	if err := db.UpdateApp(appName, updated); err != nil {
		c.JSON(http.StatusNotFound, Response{
			Status:  "error",
			Message: "App not found",
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Status:  "success",
		Message: "App updated successfully",
	})
}

func deleteAppHandler(c *gin.Context) {
	appName := c.Param("appName")

	if err := db.DeleteApp(appName); err != nil {
		c.JSON(http.StatusNotFound, Response{
			Status:  "error",
			Message: "App not found",
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Status:  "success",
		Message: "App deleted successfully",
	})
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func main() {
	db = &Database{}
	if err := db.Load(); err != nil {
		log.Fatal("Failed to load database:", err)
	}

	r := gin.Default()
	r.Use(corsMiddleware())

	// Routes
	r.GET("/health", healthHandler)

	api := r.Group("/api/v1")
	{
		api.POST("/apps", createAppHandler)
		api.GET("/apps", getAllAppsHandler)
		api.PUT("/apps/:appName", updateAppHandler)
		api.DELETE("/apps/:appName", deleteAppHandler)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8090"
	}

	log.Printf("🚀 Server running on port %s", port)
	r.Run(":" + port)
}
