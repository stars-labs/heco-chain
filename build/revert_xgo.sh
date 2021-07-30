#!/bin/sh
tagLatest=karalabe/xgo-latest:latest
tagOrigin=karalabe/xgo-latest:origin-latest
tagProxy=karalabe/xgo-latest:goproxy

if test ! -z "$(docker images -q ${tagOrigin})"; then
    if test "$(docker images -q ${tagOrigin})" = "$(docker images -q ${tagLatest})"; then
        echo "Need to do nothing"
        exit 0
    fi
    
    if test ! -z "$(docker images -q ${tagLatest})"; then
        echo "Info: remove old  ${tagLatest}"
        docker rmi ${tagLatest}
        if test ! -z "$(docker images -q ${tagProxy})"; then
            echo "Info: remove old  ${tagProxy}"
            docker rmi ${tagProxy}
        fi
    fi
    echo "Info: tag ${tagOrigin} to ${tagLatest}"
    docker tag ${tagOrigin} ${tagLatest}
    echo "Done"
else
    echo "Error: ${tagOrigin} not exist! Nothing changed."
fi

docker images karalabe/xgo-latest