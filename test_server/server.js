const express = require('express');
const fs = require('fs');
const path = require('path');
const Throttle = require('throttle');

const app = express();
const PORT = 12346;
const VIDEO_FILE = path.join(__dirname, 'sample.mp4');

// 静态文件服务：让 HLS 的 .m3u8 和 .ts 文件可以直接访问
app.use('/hls', express.static(path.join(__dirname, 'hls')));

// --- 路由 1: 支持 Range 的 MP4 ---
app.get('/video/range.mp4', (req, res) => {
    const stat = fs.statSync(VIDEO_FILE);
    const fileSize = stat.size;
    const range = req.headers.range;
    const throttle = new Throttle(300 * 1024); // 限速 300KB/s

    if (range) {
        const parts = range.replace(/bytes=/, "").split("-");
        const start = parseInt(parts[0], 10);
        const end = parts[1] ? parseInt(parts[1], 10) : fileSize - 1;
        res.writeHead(206, {
            'Content-Range': `bytes ${start}-${end}/${fileSize}`,
            'Accept-Ranges': 'bytes',
            'Content-Length': (end - start) + 1,
            'Content-Type': 'video/mp4',
        });
        fs.createReadStream(VIDEO_FILE, { start, end }).pipe(throttle).pipe(res);
    } else {
        res.writeHead(200, {
            'Content-Length': fileSize,
            'Content-Type': 'video/mp4',
            'Accept-Ranges': 'bytes',
        });
        fs.createReadStream(VIDEO_FILE).pipe(throttle).pipe(res);
    }
});

// --- 路由 2: 不支持 Range 的 MP4 (强行 200 OK) ---
app.get('/video/no-range.mp4', (req, res) => {
    const stat = fs.statSync(VIDEO_FILE);
    // 忽略请求里的 Range 只有 200 OK
    res.writeHead(200, {
        'Content-Length': stat.size,
        'Content-Type': 'video/mp4',
        // 故意不写 Accept-Ranges: bytes
    });
    const throttle = new Throttle(300 * 1024);
    fs.createReadStream(VIDEO_FILE).pipe(throttle).pipe(res);
});

// --- 综合测试网页 ---
app.get('/test-all', (req, res) => {
    res.send(`
        <!DOCTYPE html>
        <html>
        <head>
            <title>全场景视频嗅探测试</title>
            <script src="https://cdn.jsdelivr.net/npm/hls.js@latest"></script>
            <style>
                body { font-family: sans-serif; background: #222; color: #fff; padding: 20px; }
                .grid { display: grid; grid-template-columns: 1fr 1fr; gap: 20px; }
                .card { background: #333; padding: 15px; border-radius: 8px; border: 1px solid #444; }
                video { width: 100%; background: #000; margin-top: 10px; }
                h3 { margin-top: 0; color: #00d1b2; }
                .tag { font-size: 12px; background: #555; padding: 2px 6px; border-radius: 4px; }
            </style>
        </head>
        <body>
            <h1>全场景视频嗅探测试控制台</h1>
            <div class="grid">
                <!-- 1. Range MP4 -->
                <div class="card">
                    <h3>1. MP4 (支持 Range/206)</h3>
                    <span class="tag">Status: 206 Partial Content</span>
                    <video controls src="/video/range.mp4"></video>
                </div>

                <!-- 2. No Range MP4 -->
                <div class="card">
                    <h3>2. MP4 (不支持 Range/仅200)</h3>
                    <span class="tag">Status: 200 OK Only</span>
                    <video controls src="/video/no-range.mp4"></video>
                </div>

<!--                &lt;!&ndash; 3. Single HLS &ndash;&gt;-->
<!--                <div class="card">-->
<!--                    <h3>3. HLS (单分辨率 720p)</h3>-->
<!--                    <span class="tag">Type: m3u8 / single stream</span>-->
<!--                    <video id="hls-single" controls></video>-->
<!--                </div>-->

<!--                &lt;!&ndash; 4. Multi HLS &ndash;&gt;-->
<!--                <div class="card">-->
<!--                    <h3>4. HLS (多分辨率 480p/720p)</h3>-->
<!--                    <span class="tag">Type: m3u8 / Master Playlist</span>-->
<!--                    <video id="hls-multi" controls></video>-->
<!--                </div>-->
            </div>

            <script>
                function initHls(id, url) {
                    const video = document.getElementById(id);
                    if (Hls.isSupported()) {
                        const hls = new Hls();
                        hls.loadSource(url);
                        hls.attachMedia(video);
                    } else if (video.canPlayType('application/vnd.apple.mpegurl')) {
                        video.src = url;
                    }
                }
                initHls('hls-single', '/hls/single/playlist.m3u8');
                initHls('hls-multi', '/hls/multi/master.m3u8');
            </script>
        </body>
        </html>
    `);
});

app.listen(PORT, () => {
    console.log(`服务器启动: http://localhost:${PORT}/test-all`);
});