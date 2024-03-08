package handler

import (
	"encoding/json"

	"api.brutecore/internal/repository"
	"api.brutecore/libs/lib_env"
	"api.brutecore/libs/lib_jwt"

	"github.com/gofiber/fiber/v2"
)

type AUTHLayer struct {
	repo *repository.AUTHRepositoryLayer
}

func NewAUTHLayer(c *lib_env.Config, j *lib_jwt.TJWT) *AUTHLayer {
	return &AUTHLayer{
		repo: repository.NewAUTHRepositoryLayer(c, j),
	}
}

func (hl *AUTHLayer) Login(c *fiber.Ctx) error {
	data := new(repository.MLoginData)
	if err := json.Unmarshal(c.Body(), data); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "msg_txt": "Invalid JSON data",
		})
	}

	pair, err := hl.repo.LoginInDataBase(data)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false, "msg_txt": err.Error(),
		})
	} else {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"success": true, "accessToken": pair.AccessToken, "refreshToken": pair.RefreshToken,
		})
	}
}

func (hl *AUTHLayer) Logout(c *fiber.Ctx) error {
	token := c.Get("Authorization")
	err := hl.repo.LogoutUser(token)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "msg_txt": err.Error(),
		})
	} else {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"success": true, "msg_txt": "Logged Out",
		})
	}
}

func (hl *AUTHLayer) CheckTokenMiddleware(с *fiber.Ctx) error {
	err := hl.repo.CheckTokenMiddleware(с.Get("Authorization"))
	if err != nil {
		return с.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "msg_txt": err.Error(),
		})
	}
	return с.Next()
}

func (hl *AUTHLayer) CheckAdminRole(c *fiber.Ctx) error {
	err := hl.repo.CheckAdminRole(c.Get("Authorization"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "msg_txt": err.Error(),
		})
	}
	return c.Next()	
}

func (hl *AUTHLayer) CheckTokenMiddlewareQuery(с *fiber.Ctx) error {
	err := hl.repo.CheckTokenMiddleware(с.Query("auth"))
	if err != nil {
		return с.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "msg_txt": err.Error(),
		})
	}
	return с.Next()
}

func (hl *AUTHLayer) CheckAdminRoleQuery(с *fiber.Ctx) error {
	err := hl.repo.CheckAdminRole(с.Query("auth"))
	if err != nil {
		return с.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "msg_txt": err.Error(),
		})
	}
	return с.Next()
}

func (hl *AUTHLayer) Validate(c *fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true, "msg_txt": "Token is valid",
	})
}

func (hl *AUTHLayer) Refresh(c *fiber.Ctx) error {
	access := c.Get("Authorization")
	refresh := new(struct {
		Refresh string `json:"refreshToken"`
	})
	if err := json.Unmarshal(c.Body(), refresh); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "msg_txt": "Invalid JSON data",
		})
	}

	pair, err := hl.repo.RefreshToken(&lib_jwt.TokenPair{
		AccessToken: &access, RefreshToken: &refresh.Refresh,
	})
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "msg_txt": err.Error(),
		})
	} else {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"success": true, "accessToken": pair.AccessToken, "refreshToken": pair.RefreshToken,
		})
	}
}
