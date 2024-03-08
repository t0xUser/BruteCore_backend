package handler

import (
	"api.brutecore/internal/repository"
	"api.brutecore/libs/lib_db"
	"github.com/gofiber/fiber/v2"
)

type MODLLayer struct {
	repo repository.MODLRepositoryLayer
}

func NewMODLLayer(d *lib_db.DB) *MODLLayer {
	return &MODLLayer{
		repo: *repository.NewMODLRepositoryLayer(d),
	}
}

func (ml *MODLLayer) GetModules(c *fiber.Ctx) error {
	modules, err := ml.repo.GetModules()
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "msg_txt": err.Error(),
		})
	} else {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"success": true, "modules": modules,
		})
	}
}

func (ml *MODLLayer) DeleteModule(c *fiber.Ctx) error {
	err := ml.repo.DeleteModule(c.Query("id"))
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

func (ml *MODLLayer) UploadModule(c *fiber.Ctx) error {
	if err := ml.repo.UploadModule(c); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "msg_txt": err.Error(),
		})
	} else {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"success": true, "msg_txt": "Модуль добавлен",
		})
	}
}
