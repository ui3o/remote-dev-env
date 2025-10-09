const WebSocket = require("ws");
const os = require("os");
const fs = require("fs");
const path = require("path");

const get_download_folder = () => {
  const platform = os.platform(); // 'win32', 'darwin', 'linux', etc.
  switch (platform) {
    case "win32":
      return process.env.USERPROFILE + "\\Downloads";
    case "darwin":
      return process.env.HOME + "/Downloads";
    default:
      return process.env.HOME + "/Downloads";
  }
};

const downloadFolder = get_download_folder();
const files = fs
  .readdirSync(downloadFolder)
  .filter((f) => /^issh_login_data.*\.json$/.test(f))
  .map((f) => ({
    name: f,
    time: fs.statSync(path.join(downloadFolder, f)).mtime.getTime(),
  }))
  .sort((a, b) => b.time - a.time);

const latestLoginDataFile =
  files.length > 0 ? path.join(downloadFolder, files[0].name) : null;
let loginData = null;
if (latestLoginDataFile) {
  try {
    const fileContent = fs.readFileSync(latestLoginDataFile, "utf8");
    loginData = JSON.parse(fileContent);
    const ws = new WebSocket(process.argv.pop(), {
      rejectUnauthorized:false,
      headers: {
        Cookie: loginData.cookie,
      },
    });

    ws.on("open", () => {
      if (ws.readyState === WebSocket.OPEN) {
        process.stdin.on("data", (data) => {
          ws.send(data);
        });
        ws.on("message", (data) => {
          process.stdout.write(data);
        });
      } else {
        console.error("WebSocket is not open. Cannot send data.");
      }
    });

    ws.on("error", (err) => {
      console.error("WebSocket error:", err);
      process.exit(1);
    });
  } catch (err) {
    console.error("Failed to read or parse login data file:", err);
  }
}
