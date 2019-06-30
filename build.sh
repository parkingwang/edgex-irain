#!/usr/bin/env bash

modules=( "endpoint" "trigger" )

makeModule() {
    for dir in ${modules[@]} ; do
        echo "###### Building module: ${dir}"
        # Into a module dir
        cd ${dir}
        GOOS=linux GOARCH=arm make -f ../Makefile $*
        GOOS=linux GOARCH=amd64 make -f ../Makefile $*
        GOOS=linux make -f ../Makefile manifest
        cd -
    done
}

makeModule image push
