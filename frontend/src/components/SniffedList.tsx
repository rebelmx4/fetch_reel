import React, { useState, useRef, useEffect } from 'react';
import Hls from 'hls.js';
import { CreateDownloadTask, StartDownload } from '../../wailsjs/go/main/App';
import { SniffEvent, VideoTask } from '../App';

interface Props {
    items: SniffEvent[];
    onMark: (task: VideoTask) => void;
}

const SniffedList: React.FC<Props> = ({ items, onMark }) => {
    const [expandedIndex, setExpandedIndex] = useState<number | null>(null);
    const videoRef = useRef<HTMLVideoElement>(null);

    // 排序逻辑：大小降序（size 为 0 的排后面）
    const sortedItems = [...items].sort((a, b) => (b.size || 0) - (a.size || 0));

    // 处理预览播放逻辑 (HLS / MP4)
    useEffect(() => {
        if (expandedIndex !== null && videoRef.current) {
            const item = sortedItems[expandedIndex];
            // 使用我们后端 Go 写的代理服务器来绕过跨域和防盗链
            const proxyUrl = `http://127.0.0.1:12345/proxy?url=${encodeURIComponent(item.url)}&referer=${encodeURIComponent(item.originUrl)}`;

            if (item.type === 'hls' && Hls.isSupported()) {
                const hls = new Hls();
                hls.loadSource(proxyUrl);
                hls.attachMedia(videoRef.current);
            } else {
                videoRef.current.src = proxyUrl;
            }
        }
    }, [expandedIndex]);

    const handleDirectDownload = async (item: SniffEvent) => {
        try {
            // 1. 先在后端创建任务
            const task = await CreateDownloadTask(item.url, item.title, item.originUrl, item.type, item.headers);
            // 2. 直接开始下载
            await StartDownload(task.id);
            alert("已加入下载队列");
        } catch (e) {
            console.error(e);
        }
    };

    const handleMarkAction = async (item: SniffEvent) => {
        // 进入标记页面前，先创建任务对象，以便后端分配 ID 和 临时目录
        const task = await CreateDownloadTask(item.url, item.title, item.originUrl, item.type, item.headers);
        onMark(task);
    };

    return (
        <div className="flex flex-col">
            {sortedItems.length === 0 && (
                <div className="text-gray-500 text-center mt-20">暂未嗅探到资源，请在浏览器中播放视频</div>
            )}

            {sortedItems.map((item, index) => (
                <div key={index} className="border-b border-gray-800 p-3 hover:bg-gray-850 transition">
                    <div className="flex justify-between items-start">
                        <div className="flex-1 min-w-0 mr-4">
                            <div className="text-sm font-medium truncate text-blue-300" title={item.title}>
                                {item.title || "未知视频"}
                            </div>
                            <div className="text-xs text-gray-500 mt-1 truncate">{item.url}</div>
                            <div className="flex mt-2 space-x-2">
                <span className="text-[10px] bg-gray-700 px-1.5 py-0.5 rounded text-gray-300 uppercase">
                  {item.type}
                </span>
                                {item.size > 0 && (
                                    <span className="text-[10px] bg-blue-900/30 px-1.5 py-0.5 rounded text-blue-400">
                    {(item.size / 1024 / 1024).toFixed(2)} MB
                  </span>
                                )}
                            </div>
                        </div>

                        <div className="flex space-x-2">
                            <button
                                onClick={() => setExpandedIndex(expandedIndex === index ? null : index)}
                                className="p-1.5 hover:bg-gray-700 rounded text-xs"
                                title="预览"
                            >
                                {expandedIndex === index ? "收起" : "播放"}
                            </button>
                            <button
                                onClick={() => handleMarkAction(item)}
                                className="p-1.5 bg-purple-600 hover:bg-purple-700 rounded text-xs px-3"
                            >
                                标记
                            </button>
                            <button
                                onClick={() => handleDirectDownload(item)}
                                className="p-1.5 bg-green-600 hover:bg-green-700 rounded text-xs px-3"
                            >
                                全量下载
                            </button>
                        </div>
                    </div>

                    {/* 内联预览区域 */}
                    {expandedIndex === index && (
                        <div className="mt-3 bg-black rounded overflow-hidden aspect-video">
                            <video
                                ref={videoRef}
                                controls
                                autoPlay
                                className="w-full h-full"
                            />
                        </div>
                    )}
                </div>
            ))}
        </div>
    );
};

export default SniffedList;