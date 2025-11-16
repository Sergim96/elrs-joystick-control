// SPDX-FileCopyrightText: © 2023 OneEyeFPV oneeyefpv@gmail.com
// SPDX-License-Identifier: GPL-3.0-or-later
// SPDX-License-Identifier: FS-0.9-or-later

package audio

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/veandco/go-sdl2/mix"
	"github.com/veandco/go-sdl2/sdl"
	"gopkg.in/tomb.v2"
)

type Controller struct {
	baseDir string

	requests chan string
	t        *tomb.Tomb
	loopTomb *tomb.Tomb

	initOnce sync.Once
	quitOnce sync.Once
}

func NewCtl(baseDir string) *Controller {
	c := &Controller{
		baseDir:  baseDir,
		requests: make(chan string, 32),
	}
	if err := c.Init(); err != nil {
		panic(err)
	}
	return c
}

func (c *Controller) Init() error {
	var initErr error
	c.initOnce.Do(func() {
		initErr = c.initLocked()
	})
	return initErr
}

func (c *Controller) initLocked() error {
	if c.baseDir == "" {
		c.baseDir = "./audio"
	}

	var abs string
	var err error
	if abs, err = filepath.Abs(c.baseDir); err != nil {
		return err
	}
	c.baseDir = abs

	if err = os.MkdirAll(c.baseDir, 0o755); err != nil {
		return err
	}

	if err = sdl.InitSubSystem(sdl.INIT_AUDIO); err != nil {
		return err
	}

	if err = mix.Init(mix.INIT_MP3); err != nil {
		sdl.QuitSubSystem(sdl.INIT_AUDIO)
		return err
	}

	if err = mix.OpenAudio(mix.DEFAULT_FREQUENCY, mix.DEFAULT_FORMAT, mix.DEFAULT_CHANNELS, mix.DEFAULT_CHUNKSIZE); err != nil {
		mix.Quit()
		sdl.QuitSubSystem(sdl.INIT_AUDIO)
		return err
	}

	c.t = &tomb.Tomb{}
	c.loopTomb = &tomb.Tomb{}
	c.loopTomb.Go(c.loop)

	return nil
}

func (c *Controller) Quit() {
	c.quitOnce.Do(func() {
		if c.loopTomb != nil {
			c.loopTomb.Kill(nil)
			_ = c.loopTomb.Wait()
		}
		if c.t != nil {
			c.t.Kill(nil)
			_ = c.t.Wait()
		}
		if c.requests != nil {
			close(c.requests)
			c.requests = nil
		}
		mix.HaltMusic()
		mix.CloseAudio()
		mix.Quit()
		sdl.QuitSubSystem(sdl.INIT_AUDIO)
		c.loopTomb = nil
		c.t = nil
	})
}

func (c *Controller) ListMessages() ([]string, error) {
	dirEntries, err := os.ReadDir(c.baseDir)
	if err != nil {
		return nil, err
	}

	files := make([]string, 0, len(dirEntries))
	for _, entry := range dirEntries {
		if entry.IsDir() {
			continue
		}
		if strings.EqualFold(filepath.Ext(entry.Name()), ".mp3") {
			files = append(files, entry.Name())
		}
	}

	sort.Strings(files)
	return files, nil
}

func (c *Controller) Play(fileName string) error {
	if c.loopTomb == nil || c.requests == nil {
		return errors.New("audio controller not initialized")
	}

	if fileName == "" {
		return errors.New("file name is required")
	}

	fullPath := filepath.Join(c.baseDir, fileName)
	if _, err := os.Stat(fullPath); err != nil {
		return err
	}

	select {
	case c.requests <- fullPath:
		return nil
	default:
		return errors.New("audio controller busy")
	}
}

func (c *Controller) playFile(path string) error {
	music, err := mix.LoadMUS(path)
	if err != nil {
		return err
	}
	defer music.Free()

	if err = music.Play(0); err != nil {
		return err
	}

	for mix.PlayingMusic() {
		time.Sleep(25 * time.Millisecond)
	}
	return nil
}

func (c *Controller) loop() error {
	for {
		select {
		case <-c.loopTomb.Dying():
			return nil
		case path, ok := <-c.requests:
			if !ok {
				return nil
			}
			mix.HaltMusic()
			if err := c.playFile(path); err != nil {
				fmt.Printf("audio: failed to play %s: %v\n", path, err)
			}
		}
	}
}
