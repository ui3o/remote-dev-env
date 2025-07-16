#!/bin/node

const makeFile = `${process.cwd()}/Makefile.py`;
const fs = require("fs");

const printParamList = (line, comment) => {
  const funcDef = line.split("def ")[1].split("(");
  const paramList = funcDef[1].split(")")[0];
  const pl = paramList.split(",");
  const allParamCount = paramList.length ? pl.length : 0;
  console.log(`${funcDef[0]}:# ${comment.trim()}`);
  console.log(`${funcDef[0]}:@ »» (${paramList})`);
};

if (fs.existsSync(makeFile)) {
  let comment = "";
  let commentNumber = 0;
  fs.readFileSync(makeFile, "utf-8")
    .split(/\r?\n/)
    .forEach((line, index) => {
      if (line.startsWith("#")) {
        commentNumber = index;
        comment = line.replace("#", "");
      }
      if (line.match(new RegExp("^def .*:$")))
        if (index - 1 === commentNumber) {
          printParamList(line, comment);
        }
    });
}
