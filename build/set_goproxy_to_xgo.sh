#!/bin/sh
tagLatest=karalabe/xgo-latest:latest
tagOrigin=karalabe/xgo-latest:origin-latest
tagProxy=karalabe/xgo-latest:goproxy

# check if already set
if test -n "$(docker images -q ${tagProxy})"; then
    if test "$(docker images -q ${tagProxy})" = "$(docker images -q ${tagLatest})"; then
        # already set, just exit
        exit 0
    else
        echo "Using existed image ${tagProxy}"
        docker tag ${tagProxy} ${tagLatest}
        docker images karalabe/xgo-latest
        exit 0
    fi
fi

# check env GOPROXY
if [ $GOPROXY ] && [ $GOPROXY != https://proxy.golang.org,direct ]; then
    echo "Using ${GOPROXY}"
else
    echo "No custom GOPROXY, nothing changed."
    exit 0
fi

docker run -d --name=xgo-build ${tagLatest} /bin/sh
docker exec xgo-build /bin/bash -c "go env -w GOPROXY=${GOPROXY}"
docker commit -a "darlzan" -m "use ${GOPROXY}" xgo-build ${tagProxy}
docker stop xgo-build
docker rm xgo-build
docker tag ${tagLatest} ${tagOrigin}
docker tag ${tagProxy} ${tagLatest}
#show results
docker images karalabe/xgo-latest