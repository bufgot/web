package context

import (
	"github.com/bufgot/web"
	"github.com/bufgot/web/engine/chi"
	"github.com/bufgot/web/engine/echo"
	"github.com/bufgot/web/engine/fasthttp"
	"github.com/bufgot/web/engine/fiber"
	"github.com/bufgot/web/engine/gin"
	"github.com/bufgot/web/engine/hertz"
)

// NewAdapter returns the appropriate WebFramework adapter based on engine type.
func NewAdapter(engine EngineType) web.WebFramework {
	switch engine {
	case EngineChi:
		return chi.NewChiAdapter()
	case EngineGin:
		return gin.NewGinAdapter()
	case EngineEcho:
		return echo.NewEchoAdapter()
	case EngineFiber:
		return fiber.NewFiberAdapter()
	case EngineFasthttp:
		return fasthttp.NewFasthttpAdapter()
	default:
		return hertz.NewHertzAdapter()
	}
}
