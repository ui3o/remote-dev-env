#!/bin/node

const makeFile = `${process.cwd()}/Makefile`
const fs = require('fs');

if (fs.existsSync(makeFile)) {
    let comment = ''
    let commentNumber = 0;
    fs.readFileSync(makeFile, 'utf-8').split(/\r?\n/).forEach((line, index) => {
        if (line.startsWith("#")) {
            commentNumber = index;
            comment = line.replace('#', '');
        }
        if (line.match(new RegExp('^[a-zA-z].*:[ a-zA-z0-9-_]*$'))) {
            if (index - 1 === commentNumber)
                console.log(`${line.split(':')[0]}:${comment.trim()}`);
            else
                console.log(line.split(':')[0]);
        }
    });
}
