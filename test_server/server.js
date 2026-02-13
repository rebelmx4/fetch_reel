const express = require('express');
const fs = require('fs');
const path = require('path');

const app = express();
const PORT = 12346;
const VIDEO_FILE = path.join(__dirname, 'sample.mp4'); // 确保这里有 sample.mp4

// 1. 模拟动态 Token 和 Referer 校验
const videoMiddleware = (req, res, next) => {
    const referer = req.get('Referer');
    const secureToken = req.query.secure;
    const expectedToken = 'j9CIL3EuXDkSSMhwy1E30Q';

    console.log(`--- 收到请求 ---`);
    console.log(`Referer: ${referer}`);
    console.log(`Token: ${secureToken}`);

    // 暂时只打印日志，不直接返回 403，方便你调试
    if (!referer || !referer.includes('localhost')) {
        console.log(`[警告] Referer 校验不匹配，但在测试模式下允许通过`);
    }

    if (secureToken !== expectedToken) {
        console.log(`[警告] Token 校验不匹配，但在测试模式下允许通过`);
    }

    next();
};

// 2. 模拟视频文件的分段下载 (Range 请求)
app.get('/1/0/10278526-720p.mp4', videoMiddleware, (req, res) => {
    const stat = fs.statSync(VIDEO_FILE);
    const fileSize = stat.size;
    const range = req.headers.range;

    if (range) {
        const parts = range.replace(/bytes=/, "").split("-");
        const start = parseInt(parts[0], 10);
        const end = parts[1] ? parseInt(parts[1], 10) : fileSize - 1;

        const chunksize = (end - start) + 1;
        const file = fs.createReadStream(VIDEO_FILE, { start, end });

        const head = {
            'Content-Range': `bytes ${start}-${end}/${fileSize}`,
            'Accept-Ranges': 'bytes',
            'Content-Length': chunksize,
            'Content-Type': 'video/mp4',
        };

        res.writeHead(206, head); // 206 Partial Content
        file.pipe(res);
        console.log(`[分段] 发送字节: ${start}-${end}`);
    } else {
        const head = {
            'Content-Length': fileSize,
            'Content-Type': 'video/mp4',
        };
        res.writeHead(200, head); // 200 OK
        fs.createReadStream(VIDEO_FILE).pipe(res);
        console.log('[全量] 发送整个文件');
    }
});

// 3. 模拟视频所在的网页 (aaaaa)
app.get('/page', (req, res) => {
    const videoUrl = `http://localhost:${PORT}/1/0/10278526-720p.mp4?secure=j9CIL3EuXDkSSMhwy1E30Q`;

    res.send(`
        <!DOCTYPE html>
        <html>
        <head>
            <title>隐藏视频测试页</title>
            <style>
                /* 隐藏技巧：将视频放在屏幕外，或者设为 1像素+透明 */
                .hidden-video {
                    position: absolute;
                    left: -9999px;
                    top: -9999px;
                    width: 1px;
                    height: 1px;
                    opacity: 0;
                }
                body { font-family: sans-serif; background: #f0f0f0; padding: 50px; text-align: center; }
                .status-card { background: white; padding: 20px; border-radius: 8px; shadow: 0 2px 10px rgba(0,0,0,0.1); }
            </style>
        </head>
        <body>
            <div class="status-card">
<!--                <h1>正在静默加载视频...</h1>-->
<!--                <p>页面上看不见播放器，但你的嗅探器应该能抓到请求。</p>-->
<!--                <p>目标 URL 包含: <code>/1/0/10278526-720p.mp4</code></p>-->
                <div id="status">状态: 等待浏览器发起请求...</div>
            </div>

            <!-- 虽然看不见，但浏览器依然会执行加载逻辑 -->
            <video class="hidden-video" controls autoplay muted playsinline>
                <source src="${videoUrl}" type="video/mp4">
            </video>

            <script>
                const v = document.querySelector('video');
                v.onplay = () => {
                    document.getElementById('status').innerText = '状态: 视频已开始静默播放，流量正在发出';
                    console.log('Video is playing in background...');
                };
            </script>
        </body>
        </html>
    `);
});

// 4. 专门用于嗅探测试的网页
app.get('/sniff-test', (req, res) => {
    // 构造视频 URL
    const videoUrl = `http://localhost:${PORT}/1/0/10278526-720p.mp4?secure=j9CIL3EuXDkSSMhwy1E30Q`;

    res.send(`
        <!DOCTYPE html>
        <html>
        <head>
            <title>视频嗅探测试页 (Range Support)</title>
            <style>
                body { font-family: sans-serif; display: flex; flex-direction: column; align-items: center; padding-top: 50px; background: #1a1a1a; color: #eee; }
                .container { width: 80%; max-width: 800px; text-align: center; }
                video { width: 100%; border: 2px solid #444; border-radius: 8px; box-shadow: 0 10px 30px rgba(0,0,0,0.5); }
                .info { margin-top: 20px; padding: 15px; background: #333; border-radius: 5px; text-align: left; }
                code { color: #f39c12; }
            </style>
        </head>
        <body>
            <div class="container">
                <h1>视频嗅探测试</h1>
                <p>该页面包含一个标准播放器，服务器已开启 <b>Range (206)</b> 支持。</p>
                
                <!-- 标准播放器：嗅探器最容易识别 -->
                <video id="player" controls playsinline>
                    <source src="${videoUrl}" type="video/mp4">
                    您的浏览器不支持 HTML5 视频。
                </video>

                <div class="info">
                    <p>💡 <b>测试说明：</b></p>
                    <ul>
                        <li>点击播放后，观察嗅探器插件是否弹出下载浮窗。</li>
                        <li>服务器会针对此请求返回 <code>Accept-Ranges: bytes</code>。</li>
                        <li>当你在进度条拖动时，服务器将返回 <code>206 Partial Content</code> 状态码。</li>
                    </ul>
                </div>
            </div>

            <script>
                const video = document.getElementById('player');
                video.onplay = () => console.log('开始播放，嗅探器应已捕捉到请求');
            </script>
        </body>
        </html>
    `);
});


app.listen(PORT, () => {
    console.log(`\n==================================================`);
    console.log(`🚀 视频嗅探测试服务器已启动！`);
    console.log(`监听端口: ${PORT}`);
    console.log(`--------------------------------------------------`);
    console.log(`1️⃣  嗅探测试页 (标准播放器): http://localhost:${PORT}/sniff-test`);
    console.log(`2️⃣  静默加载页 (隐藏播放器): http://localhost:${PORT}/page`);
    console.log(`3️⃣  视频直链 (带Token):     http://localhost:${PORT}/1/0/10278526-720p.mp4?secure=j9CIL3EuXDkSSMhwy1E30Q`);
    console.log(`==================================================\n`);
    console.log(`提示: 如果嗅探器工作正常，点击播放后终端应持续打印 [分段] 日志。`);
});