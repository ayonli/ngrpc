const fs = require("fs");
const path = require("path");

const files1 = fs.readdirSync(".");

for (const file of files1) {
    if (/(.js|\.js\.map|\.d\.ts)$/.test(file) &&
        file !== path.basename(__filename) &&
        file !== "pm2.config.js"
    ) {
        fs.unlinkSync(path.join(__dirname, file));
    }
}

const files2 = fs.readdirSync("./services");

for (const file of files2) {
    if (/(.js|\.js\.map|\.d\.ts)$/.test(file) && file !== "UserService.d.ts") {
        fs.unlinkSync(path.join(__dirname, "services", file));
    }
}
