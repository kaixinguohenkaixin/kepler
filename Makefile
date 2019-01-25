#Copyright (c) 2019 VNT Chain 
all: native docker

native: kepler

kepler:
	mkdir -p build/bin
	GOBIN=$(abspath ./build/bin) go install ./node/ 
	mv ./build/bin/node ./build/bin/kepler

docker: kepler-docker

kepler-docker:
	mkdir -p build/docker/bin
	
	docker run -i --rm \
	-v $(abspath .):/opt/gopath/src/github.com/vntchain/kepler \
	-v $(abspath build/docker/bin):/opt/gopath/bin \
	-w /opt/gopath/src/github.com/vntchain/kepler \
	hyperledger/fabric-baseimage:x86_64-0.4.5 \
	go install ./node
	mv build/docker/bin/node build/docker/bin/kepler

	@echo 'Creating config'
	(cd $(GOPATH)/src/github.com/vntchain/kepler/ && tar -jc config)> build/config.tar.bz2
	(cd $(GOPATH)/src/github.com/vntchain/kepler && cp image/Dockerfile build/)
	docker build -t kepler:latest -f build/Dockerfile build/

clean:
	rm -rf build/*
	docker rmi -f kepler