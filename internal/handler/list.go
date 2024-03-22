package handler

import (
	"time"

	"api.brutecore/internal/repository"
	"api.brutecore/libs/lib_db"
	"github.com/gofiber/fiber/v2"
)

type LISTLayer struct {
	repo repository.LISTRepositoryLayer
}

func NewLISTLayer(d *lib_db.DB) *LISTLayer {
	return &LISTLayer{
		repo: *repository.NewLISTRepositoryLayer(d),
	}
}

func (ll *LISTLayer) GetComboLists(c *fiber.Ctx) error {
	tree, err := ll.repo.GetComboList()
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "msg_txt": err.Error(),
		})
	} else {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"success": true, "tree": tree,
		})
	}
}

func (ll *LISTLayer) GetInfoComboList(c *fiber.Ctx) error {
	info, err := ll.repo.GetInfoComboList(c.Query("id"))
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

func (ll *LISTLayer) DeleteComboList(c *fiber.Ctx) error {
	err := ll.repo.DeleteComboList(c.Query("id"))
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

func (ll *LISTLayer) UploadComboList(c *fiber.Ctx) error {
	if id, err := ll.repo.UploadComboListFormData(c); err != nil {
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
