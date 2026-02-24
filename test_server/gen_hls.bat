@echo off
setlocal enabledelayedexpansion

:: --- 步骤 1: 创建单分辨率 HLS (720p) ---
echo [*] 正在处理单分辨率 HLS...
if not exist "hls\single" mkdir "hls\single"
ffmpeg -i sample.mp4 -c copy -start_number 0 -hls_time 10 -hls_list_size 0 -f hls hls/single/playlist.m3u8

:: --- 步骤 2: 创建多分辨率 HLS ---
echo [*] 正在处理多分辨率分片 (480p)...
if not exist "hls\multi\480p" mkdir "hls\multi\480p"
ffmpeg -i sample.mp4 -vf "scale=-2:480" -c:v libx264 -b:v 800k -hls_time 10 -hls_list_size 0 hls/multi/480p/index.m3u8

echo [*] 正在处理多分辨率分片 (720p)...
if not exist "hls\multi\720p" mkdir "hls\multi\720p"
ffmpeg -i sample.mp4 -vf "scale=-2:720" -c:v libx264 -b:v 1500k -hls_time 10 -hls_list_size 0 hls/multi/720p/index.m3u8

:: --- 步骤 3: 生成 Master Playlist ---
echo [*] 正在生成 master.m3u8...
set "master=hls\multi\master.m3u8"
echo #EXTM3U > "%master%"
echo #EXT-X-STREAM-INF:BANDWIDTH=800000,RESOLUTION=854x480 >> "%master%"
echo 480p/index.m3u8 >> "%master%"
echo #EXT-X-STREAM-INF:BANDWIDTH=1500000,RESOLUTION=1280x720 >> "%master%"
echo 720p/index.m3u8 >> "%master%"

echo.
echo [OK] 转换完成！
echo 请运行 node server.js 并访问 http://localhost:12346/test-all
pause