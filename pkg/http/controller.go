// SPDX-FileCopyrightText: © 2023 OneEyeFPV oneeyefpv@gmail.com
// SPDX-License-Identifier: GPL-3.0-or-later
// SPDX-License-Identifier: FS-0.9-or-later

package http

import (
	"context"
	"errors"
	"fmt"
	"github.com/improbable-eng/grpc-web/go/grpcweb"
	ac "github.com/kaack/elrs-joystick-control/pkg/audio"
	bt "github.com/kaack/elrs-joystick-control/pkg/headtrackerbt"
	"github.com/kaack/elrs-joystick-control/webapp"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/ttys3/echo-pprof/v4"

	"google.golang.org/grpc"
	"gopkg.in/tomb.v2"

	"net/http"
	"time"
)

type Controller struct {
	webAppPort int
	httpTomb   *tomb.Tomb
	echo       *echo.Echo
	gRPCServer *grpc.Server
	audioCtl   *ac.Controller
}

func NewCtl(webAppPort int, gRPCServer *grpc.Server, audioCtl *ac.Controller) *Controller {
	httpCtl := &Controller{
		webAppPort: webAppPort,
		gRPCServer: gRPCServer,
		audioCtl:   audioCtl,
	}

	if err := httpCtl.Init(); err != nil {
		panic(err)
	}

	return httpCtl
}

func (c *Controller) Init() (err error) {

	if err = c.Start(); err != nil {
		return errors.Join(errors.New("could not start http server"), err)
	}

	return nil
}

func (c *Controller) NewEcho(err error) (*echo.Echo, error) {
	var httpFS http.FileSystem
	if httpFS, err = webapp.HTTPFileSystem(); err != nil {
		return nil, err
	}

	echoServer := echo.New()

	echoHandler := echoServer
	wrappedGrpc := grpcweb.WrapServer(c.gRPCServer)
	echopprof.Wrap(echoHandler)

	//override server handler to intercept grpc-web requests (content-type: application/grpc-web)
	echoServer.Server = &http.Server{
		Addr: fmt.Sprintf(":%d", c.webAppPort),
		Handler: http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			resp.Header().Set("Access-Control-Allow-Headers", "*")
			resp.Header().Set("Access-Control-Allow-Origin", "*")

			if wrappedGrpc.IsGrpcWebRequest(req) {
				wrappedGrpc.ServeHTTP(resp, req)
				return
			}

			echoHandler.ServeHTTP(resp, req)
		}),
	}

	echoServer.Use(middleware.StaticWithConfig(middleware.StaticConfig{
		Filesystem: httpFS,
		HTML5:      true,
	}))

	c.registerRoutes(echoHandler)

	echoServer.HideBanner = true
	return echoServer, nil
}

func (c *Controller) Start() (err error) {
	if c.httpTomb != nil && c.httpTomb.Alive() {
		return errors.New("http already started")
	}

	c.echo, err = c.NewEcho(err)
	if err != nil {
		return err
	}

	c.httpTomb = &tomb.Tomb{}
	c.httpTomb.Go(func() error {

		fmt.Printf("⇨ http server started on [::]%s\n", c.echo.Server.Addr)
		if err := c.echo.Server.ListenAndServe(); err != http.ErrServerClosed {
			fmt.Println("(http): server halted forcefully")
			return err
		}

		fmt.Println("(http): server halted gracefully")
		return nil
	})

	c.httpTomb.Go(func() error {
		<-c.httpTomb.Dying()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := c.echo.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return nil
	})

	return nil
}

func (c *Controller) Stop() (err error) {
	if c.httpTomb == nil || !c.httpTomb.Alive() {
		return nil
	}

	c.httpTomb.Kill(nil)
	if err := c.httpTomb.Wait(); err != nil {
		return err
	}
	return nil
}

func (c *Controller) Quit() {
	if err := c.Stop(); err != nil {
		fmt.Printf("error while exiting http controller. %s\n", err.Error())
	}
}

func (c *Controller) registerRoutes(server *echo.Echo) {
	server.GET("/api/audio/messages", c.getAudioMessages)
	server.GET("/api/bluetooth/devices", c.getBluetoothDevices)
	server.POST("/api/bluetooth/scan", c.scanBluetoothDevices)
	server.POST("/api/bluetooth/connect", c.connectBluetoothDevice)
}

func (c *Controller) getAudioMessages(ctx echo.Context) error {
	files := []string{}
	if c.audioCtl != nil {
		var err error
		if files, err = c.audioCtl.ListMessages(); err != nil {
			return ctx.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
	}
	return ctx.JSON(http.StatusOK, map[string][]string{"files": files})
}

func (c *Controller) getBluetoothDevices(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, bt.DefaultManager.Snapshot())
}

func (c *Controller) scanBluetoothDevices(ctx echo.Context) error {
	type scanReq struct {
		Seconds int `json:"seconds"`
	}

	req := scanReq{Seconds: 6}
	_ = ctx.Bind(&req)
	if req.Seconds <= 0 {
		req.Seconds = 6
	}
	if req.Seconds > 30 {
		req.Seconds = 30
	}

	scanCtx, cancel := context.WithTimeout(ctx.Request().Context(), time.Duration(req.Seconds+5)*time.Second)
	defer cancel()

	state := bt.DefaultManager.Scan(scanCtx, time.Duration(req.Seconds)*time.Second)
	if state.LastScanError != "" {
		return ctx.JSON(http.StatusBadGateway, state)
	}
	return ctx.JSON(http.StatusOK, state)
}

func (c *Controller) connectBluetoothDevice(ctx echo.Context) error {
	type connectReq struct {
		Address string `json:"address"`
	}

	var req connectReq
	if err := ctx.Bind(&req); err != nil {
		return ctx.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	if req.Address == "" {
		return ctx.JSON(http.StatusBadRequest, map[string]string{"error": "address is required"})
	}

	state, ok := bt.DefaultManager.Select(req.Address)
	if !ok {
		return ctx.JSON(http.StatusNotFound, map[string]any{
			"error": "device not found in scanned list",
			"state": state,
		})
	}

	return ctx.JSON(http.StatusOK, state)
}
