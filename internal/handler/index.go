package handler

import (
	eg "api.brutecore/internal/engine"
	"api.brutecore/internal/utility"

	"api.brutecore/libs/lib_db"
	"api.brutecore/libs/lib_env"
	"api.brutecore/libs/lib_jwt"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	conf     *lib_env.Config
	authIntr *AUTHLayer
	listIntr *LISTLayer
	proxIntr *PROXLayer
	sessIntr *SESSLayer
	modlIntr *MODLLayer
}

func New(cf *lib_env.Config, dbs *lib_db.DB, jwt *lib_jwt.TJWT, pl *eg.Pulling) *Handler {
	return &Handler{
		conf:     cf,
		authIntr: NewAUTHLayer(cf, jwt),
		listIntr: NewLISTLayer(dbs),
		proxIntr: NewPROXLayer(dbs),
		sessIntr: NewSESSLayer(dbs, pl),
		modlIntr: NewMODLLayer(dbs),
	}
}

func (h *Handler) Health(c *fiber.Ctx) error {
	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"success": true,
		"status":  "alive",
		"ram":     utility.GetRAMUsage(),
		"cpu":     utility.GetCPUUsage(),
		"disk":    utility.GetDiskUsage(),
	})
}

func (h *Handler) SetAuthHandlers(app *fiber.App) {
	/*----------- AUTH Endpoints -----------*/
	auth := app.Group(h.conf.Http.Group + "/auth")
	auth.Post("/Login", h.authIntr.Login)
	auth.Use(h.authIntr.CheckTokenMiddleware)
	auth.Post("/Logout", h.authIntr.Logout)
	auth.Post("/Validate", h.authIntr.Validate)
	auth.Post("/Refresh", h.authIntr.Refresh)
}

func (h *Handler) SetComboListHandlers(app *fiber.App) {
	/*----------- ComboList Endpoints -----------*/
	list := app.Group(h.conf.Http.Group + "/list")
	list.Use(h.authIntr.CheckTokenMiddleware)
	list.Get("/GetComboLists", h.listIntr.GetComboLists)
	list.Get("/GetInfoComboList", h.listIntr.GetInfoComboList)
	list.Use(h.authIntr.CheckAdminRole)
	list.Get("/DeleteComboList", h.listIntr.DeleteComboList)
	list.Post("/UploadComboList", h.listIntr.UploadComboList)
}

func (h *Handler) SetProxyPresetHandlers(app *fiber.App) {
	/*----------- ProxyPreset Endpoints -----------*/
	prox := app.Group(h.conf.Http.Group + "/prox")
	prox.Use(h.authIntr.CheckTokenMiddleware)
	prox.Get("/GetProxyPresets", h.proxIntr.GetProxyPresets)
	prox.Get("/GetInfoProxyPreset", h.proxIntr.GetInfoProxyPreset)
	prox.Use(h.authIntr.CheckAdminRole)
	prox.Get("/DeleteProxyPreset", h.proxIntr.DeleteProxyPreset)
	prox.Post("/UploadProxyPreset", h.proxIntr.UploadProxyPreset)
}

func (h *Handler) SetModuleHandlers(app *fiber.App) {
	/*----------- Module Endpoints -----------*/
	modl := app.Group(h.conf.Http.Group + "/modl")
	modl.Use(h.authIntr.CheckTokenMiddleware)
	modl.Get("/GetModules", h.modlIntr.GetModules)
	modl.Use(h.authIntr.CheckAdminRole)
	modl.Get("/DeleteModule", h.modlIntr.DeleteModule)
	modl.Post("/UploadModule", h.modlIntr.UploadModule)
}

func (h *Handler) SetSessionHandlers(app *fiber.App) {
	/*----------- Sessions Endpoints -----------*/
	sess := app.Group(h.conf.Http.Group + "/sess")
	sess.Use(h.authIntr.CheckTokenMiddleware)
	sess.Get("/GetSessions", h.sessIntr.GetSessions)
	sess.Get("/GetInfoSession", h.sessIntr.GetInfoSession)
	sess.Get("/GetStatistic", h.sessIntr.GetStatistic)
	sess.Get("/GetResults", h.sessIntr.GetResults)
	sess.Use(h.authIntr.CheckAdminRole)
	sess.Post("/CreateSession", h.sessIntr.CreateSession)
	sess.Get("/DeleteSession", h.sessIntr.DeleteSession)
	sess.Post("/AlterSession", h.sessIntr.AlterSession)
	sess.Post("/ApplyInputFields", h.sessIntr.ApplyInputFields)
	sess.Post("/UploadProxy", h.sessIntr.UploadProxy)
}

func (h *Handler) SetSessionDownloadHandlers(app *fiber.App) {
	/*-------- Sessions Download Endpoints --------*/
	dwnl := app.Group(h.conf.Http.Group + "/dwnl")
	dwnl.Use(h.authIntr.CheckTokenMiddlewareQuery)
	dwnl.Use(h.authIntr.CheckAdminRoleQuery)
	dwnl.Get("/DownloadSelected", h.sessIntr.DownloadUniversal)
	dwnl.Get("/DownloadAll", h.sessIntr.DownloadUniversal)
}

func (h *Handler) SetHandlers(app *fiber.App) {
	app.Get("/Health", h.Health)
	app.Get("/Dashboard", func(c *fiber.Ctx) error {
		return c.Redirect("/", fiber.StatusMovedPermanently)
	})

	h.SetAuthHandlers(app)
	h.SetComboListHandlers(app)
	h.SetProxyPresetHandlers(app)
	h.SetModuleHandlers(app)
	h.SetSessionHandlers(app)
	h.SetSessionDownloadHandlers(app)
}
