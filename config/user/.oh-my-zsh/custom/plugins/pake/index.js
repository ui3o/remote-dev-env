#!/bin/node

const makeFile = `${process.cwd()}/Makefile.py`;
const fs = require("fs");

const printParamList = (line, comment) => {
  const funcDef = line.split("def ")[1].split("(");
  const paramList = funcDef[1].split(")")[0];
  const pl = paramList.split(",");
  const allParamCount = paramList.length ? pl.length : 0;
  if (paramList.includes("=")) {
    let allParamList = {};
    let firstDefaultPos = paramList.split("=")[0].split(",").length - 1;
    let i = paramList.split("=").length - 1;
    const allDefaultCount = paramList.split("=").length - 1;
    for (; i; i--, firstDefaultPos++) {
      // allParamList[`.${allParamCount}${allDefaultCount}${i}`] = pl.map((v, index) => {
      allParamList[`.${allParamCount}${i}`] = pl.map((v, index) => {
        if (index < firstDefaultPos) {
          return v.split("=")[0].trim();
        }
        return v.split("=")[1].trim();
      });
    }
    // allParamList[`.${allParamCount}${allDefaultCount}0`] = pl.map((v) => v.split("=")[0].trim());
    allParamList[`.${allParamCount}0`] = pl.map((v) => v.split("=")[0].trim());
    Object.keys(allParamList)
      .reverse()
      .forEach((k) => {
        console.log(`${funcDef[0] + k}:${comment.trim()} => (${allParamList[k].join(", ")})`);
      });
  } else {
    console.log(`${funcDef[0]}.${allParamCount}0:${comment.trim()} => (${paramList})`);
  }
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
