package main

import (
	"context"
	"flag"
	"log"
	"time"

	_ "github.com/rclone/rclone/backend/b2"
	_ "github.com/rclone/rclone/backend/crypt"
	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/fs"
	"k8s.io/utils/inotify"
)

var (
	watchDir = flag.String("dir", "", "directory to watch for file uploads")
	dstPath  = flag.String("dst", "", "directory to upload to")
	srcPath  = flag.String("src", "", "directory to upload from")
)

func main() {
	flag.Parse()
	log.SetFlags(log.Flags() | log.Lshortfile)

	if err := run(); err != nil {
		log.Fatalf("%+v", err)
	}
}

func upload(ctx context.Context, src, dst fs.Fs, file string) error {
	ctx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()

	log.Printf("uploading %q", src)

	obj, err := src.NewObject(ctx, file)
	if err != nil {
		return err
	}
	f, err := obj.Open(ctx)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := dst.Put(ctx, f, obj); err != nil {
		return err
	}
	return nil
}

func run() error {
	watcher, err := inotify.NewWatcher()
	if err != nil {
		return err
	}
	if err := watcher.AddWatch(*watchDir, inotify.InCloseWrite); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	src, err := fs.NewFs(ctx, *srcPath)
	if err != nil {
		log.Fatal(err)
	}

	dst, err := fs.NewFs(ctx, *dstPath)
	if err != nil {
		log.Fatal(err)
	}

	for {
		select {
		case ev := <-watcher.Event:
			log.Printf("event: %+v", ev)
			if err := upload(ctx, src, dst, ev.Name); err != nil {
				return err
			}
		case err := <-watcher.Error:
			return err
		}
	}
}
