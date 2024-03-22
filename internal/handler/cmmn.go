package handler

import (
	"api.brutecore/internal/utility"
	"api.brutecore/libs/lib_db"
	"github.com/gofiber/fiber/v2"
)

func (h *Handler) Health(c *fiber.Ctx) error {
	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"success": true,
		"status":  "alive",
		"ram":     utility.GetRAMUsage(),
		"cpu":     utility.GetCPUUsage(),
		"disk":    utility.GetDiskUsage(),
	})
}

func (h *Handler) GetReference(c *fiber.Ctx) error {
	group := c.Query("group")
	var (
		err error
		res *lib_db.DBResult
	)
	if group == "" {
		res, err = h.db.QueryRow(lib_db.TxRead, "select * from reference r order by r.'group', r.id")
	} else {
		res, err = h.db.QueryRow(lib_db.TxRead, "select * from reference r where r.'group' = $1 order by r.'group', r.id", group)
	}

	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "msg_txt": err.Error(),
		})
	} else {
		return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
			"success": true, "refs": res,
		})
	}
}

func (h *Handler) GetInfo(c *fiber.Ctx) error {
	var info []map[string]string

	info = append(info, map[string]string{"key": "Версия ядра БД", "val": "4"})
	info = append(info, map[string]string{"key": "Версия ПО", "val": "1.0.0.3"})
	info = append(info, map[string]string{"key": "Автор", "val": "0xUser"})
	info = append(info, map[string]string{"key": "Дата последнего обновления", "val": "21.03.2024"})

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"success": true, "info": info,
	})
}

func (h *Handler) DashBoard(c *fiber.Ctx) error {
	return c.Redirect("/", fiber.StatusMovedPermanently)
}
