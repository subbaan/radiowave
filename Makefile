.PHONY: build
build:
	cd src/radiowave-src && go build -buildvcs=false -trimpath -o radiowave .
	ln -sfn src/radiowave-src/radiowave radiowave
