package http

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/fidellr/jastip_way/backend/plateu"
	"github.com/fidellr/jastip_way/backend/plateu/models"
	"github.com/labstack/echo"
)

type imageHandler struct {
	service plateu.ImageUsecase
}

type imageRequirements func(d *imageHandler)

func NewImageHandler(e *echo.Echo, reqs ...imageRequirements) {
	handler := new(imageHandler)

	for _, req := range reqs {
		req(handler)
	}

	e.POST("/image/upload/:person_name", handler.StoreImage)
	e.GET("/images", handler.FetchImages)
	e.GET("/image/:id", handler.GetImageByID)
	e.PUT("/image/:id", handler.UpdateImageByID)
}

func ImageService(service plateu.ImageUsecase) imageRequirements {
	return func(h *imageHandler) {
		h.service = service
	}
}

func (h *imageHandler) StoreImage(c echo.Context) (err error) {
	image := new(models.Image)
	image.PersonName = c.Param("person_name")
	needs := c.FormValue("needs")
	image.Needs = needs

	ctx := c.Request().Context()
	if ctx == nil {
		ctx = context.Background()
	}

	if image.PersonName == "" {
		return plateu.ConstraintErrorf("Failed to read person name, person name is [%s] while its required", image.PersonName)
	}

	if strings.Contains(image.PersonName, " ") {
		image.FileLink = fmt.Sprintf("%s", strings.Join(strings.Split(strings.ToLower(image.PersonName), " "), "_"))
	}

	done := make(chan bool)
	log.Println("Upload starting...")
	go func() {
		err = UploadFile(c, image, needs)
		if err != nil {
			done <- true
			return
		}

		err = h.service.StoreImage(ctx, image)
		done <- true
		return
	}()

	if <-done {
		if err != nil {
			log.Printf("Upload interrupted with error : %s", err.Error())
			return c.NoContent(http.StatusUnprocessableEntity)
		}

		log.Println("Upload finished...")
		return c.NoContent(http.StatusCreated)
	}

	return c.NoContent(http.StatusInternalServerError)
}

func (h *imageHandler) FetchImages(c echo.Context) error {
	ctx := c.Request().Context()
	if ctx == nil {
		ctx = context.Background()
	}

	var num int
	if c.QueryParam("num") != "" {
		var err error
		num, err = strconv.Atoi(c.QueryParam("num"))
		if err != nil {
			return plateu.ConstraintErrorf("%s", err.Error())
		}
	}

	filter := &plateu.Filter{
		Cursor:   c.QueryParam("cursor"),
		Num:      num,
		RoleName: c.QueryParam("role"),
	}

	images, nextCursor, err := h.service.FetchImages(ctx, filter)
	if err != nil {
		return plateu.ConstraintErrorf("%s", err.Error())
	}

	c.Response().Header().Set("X-Cursor", nextCursor)
	return c.JSON(http.StatusOK, images)
}

// TEST AND IMPLEMENT DECOMPRESSION OF GZIP

func (h *imageHandler) GetImageByID(c echo.Context) (err error) {
	ctx := c.Request().Context()
	if ctx == nil {
		ctx = context.Background()
	}

	imgID := c.Param("id")
	if imgID == "" {
		return plateu.ConstraintErrorf("%s", err.Error())
	}

	image, err := h.service.GetImageByID(ctx, imgID)
	if err != nil {
		return plateu.ConstraintErrorf("Failed to get image with id %s : %s", imgID, err.Error())
	}

	return c.JSON(http.StatusOK, image)
}

func (h *imageHandler) UpdateImageByID(c echo.Context) (err error) {
	ctx := c.Request().Context()
	if ctx == nil {
		ctx = context.Background()
	}

	imgID := c.Param("id")
	if imgID == "" {
		return plateu.ConstraintErrorf("Failed to get image id : %s", err.Error())
	}

	m := new(models.Image)
	if err = c.Bind(m); err != nil {
		return plateu.ConstraintErrorf("Failed to bind image model : %s", err.Error())
	}

	err = h.service.UpdateImageByID(ctx, imgID, m)
	if err != nil {
		return plateu.ConstraintErrorf("Failed to update image by id %s : %s", imgID, err.Error())
	}

	return c.JSON(http.StatusOK, true)
}
