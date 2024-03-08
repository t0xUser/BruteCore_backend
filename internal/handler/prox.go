package handler

import (
	"encoding/json"
	"time"

	"api.brutecore/internal/repository"
	"api.brutecore/libs/lib_db"
	"github.com/gofiber/fiber/v2"
)

type PROXLayer struct {
	repo repository.PROXRepositoryLayer
}

func NewPROXLayer(d *lib_db.DB) *PROXLayer {
	return &PROXLayer{
		repo: *repository.NewPROXRepositoryLayer(d),
	}
}

func (pl *PROXLayer) GetProxyPresets(c *fiber.Ctx) error {
	presets, err := pl.repo.GetProxyPresets()
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "msg_txt": err.Error(),
		})
	} else {
		return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
			"success": true, "presets": presets,
		})
	}
}

func (pl *PROXLayer) GetInfoProxyPreset(c *fiber.Ctx) error {
	info, err := pl.repo.GetInfoProxyPreset(c.Query("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "msg_txt": err.Error(),
		})
	} else {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"success": true, "info": info,
		})
	}
}

func (pl *PROXLayer) DeleteProxyPreset(c *fiber.Ctx) error {
	err := pl.repo.DeleteProxyPreset(c.Query("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "msg_txt": err.Error(),
		})
	} else {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"success": true, "msg_txt": "Элемент удален",
		})
	}
}

func (pl *PROXLayer) UploadProxyPreset(c *fiber.Ctx) error {
	data := new(repository.MProxyPreset)
	if err := json.Unmarshal(c.Body(), data); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "msg_txt": err.Error(),
		})
	}

	id, err := pl.repo.UploadProxyPreset(data)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "msg_txt": err.Error(),
		})
	} else {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"success": true, "id": id,
			"create_time": time.Now().Format("2006-01-02T15:04:05Z"),
		})
	}
}
