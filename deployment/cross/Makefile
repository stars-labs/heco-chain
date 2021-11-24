PROJECT_ROOT_DIR = ../..
ENTRY_FILE_GETH_DIR = github.com/ethereum/go-ethereum/cmd/geth
ENTRY_FILE_BOOTNODE_DIR = github.com/ethereum/go-ethereum/cmd/bootnode
GROUP_NAME = hecochain.com
SERVICE_NAME_BOOTNODE = bootnode
SERVICE_NAME_MINER = miner
BIN_REVISION_STRING := $(shell git show -s --pretty=format:%h)
GIT_TIME := $(shell git show -s --pretty=format:%cI)
GO_PROXY := $(shell go env GOPROXY)

build_linux_node:
	xgo2 --goproxy="${GO_PROXY}" --targets=linux/amd64 -ldflags "-s -w -X 'main.gitCommit=${BIN_REVISION_STRING}' -X 'main.gitDate=${GIT_TIME}'" -out geth --pkg=${ENTRY_FILE_GETH_DIR} ${PROJECT_ROOT_DIR}

devtools:
	go get -u github.com/stars-labs/xgo2

all: docker_bootnode docker_miner

build_linux_bootnode:
	xgo2 --targets=linux/amd64 -ldflags "-s -w" -out bootnode --pkg=${ENTRY_FILE_BOOTNODE_DIR} ${PROJECT_ROOT_DIR}

docker_bootnode:build_linux_bootnode
	docker build -f ./Dockerfile-Bootnode -t ${GROUP_NAME}/${SERVICE_NAME_BOOTNODE}:${BIN_REVISION_STRING} .
	docker tag ${GROUP_NAME}/${SERVICE_NAME_BOOTNODE}:${BIN_REVISION_STRING} ${GROUP_NAME}/${SERVICE_NAME_BOOTNODE}:latest

docker_miner:build_linux_node
	docker build -f ./Dockerfile-Miner -t ${GROUP_NAME}/${SERVICE_NAME_MINER}:${BIN_REVISION_STRING} .
	docker tag ${GROUP_NAME}/${SERVICE_NAME_MINER}:${BIN_REVISION_STRING} ${GROUP_NAME}/${SERVICE_NAME_MINER}:latest

clean:
	rm -rf geth-* bootnode-*
