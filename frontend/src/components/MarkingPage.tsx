import React, { useState, useRef, useEffect } from 'react';
import Hls from 'hls.js';
import { StartDownload } from '../../wailsjs/go/main/App';
import { VideoTask, Clip } from '../App';

interface Props {
    task: VideoTask;
    onClose: () => void;
}

const MarkingPage: React.FC<Props> = ({ task, onClose }) => {
    const [duration, setDuration] = useState(0);
    const [currentTime, setCurrentTime] = useState(0);
    const [clips, setClips] = useState<Clip[]>([]);
    const [selectedClipId, setSelectedClipId] = useState<number | null>(null);

    const videoRef = useRef<HTMLVideoElement>(null);
    const hlsRef = useRef<Hls | null>(null);

    // 1. 初始化视频和轨道
    useEffect(() => {
        const video = videoRef.current;
        if (!video) return;

        const proxyUrl = `http://127.0.0.1:12345/proxy?url=${encodeURIComponent(task.url)}&referer=${encodeURIComponent(task.originUrl)}`;

        if (task.type === 'hls' && Hls.isSupported()) {
            const hls = new Hls();
            hls.loadSource(proxyUrl);
            hls.attachMedia(video);
            hlsRef.current = hls;
        } else {
            video.src = proxyUrl;
        }

        const onLoadedMetadata = () => {
            const d = video.duration;
            setDuration(d);
            // 初始状态：一个完整的片段代表整个视频
            setClips([{ id: Date.now(), start: 0, end: d, status: 'keep' }]);
        };

        const onTimeUpdate = () => setCurrentTime(video.currentTime);

        video.addEventListener('loadedmetadata', onLoadedMetadata);
        video.addEventListener('timeupdate', onTimeUpdate);

        // 监听键盘快捷键
        window.addEventListener('keydown', handleKeyDown);

        return () => {
            video.removeEventListener('loadedmetadata', onLoadedMetadata);
            video.removeEventListener('timeupdate', onTimeUpdate);
            window.removeEventListener('keydown', handleKeyDown);
            if (hlsRef.current) hlsRef.current.destroy();
        };
    }, [task]);

    // 2. 快捷键逻辑
    const handleKeyDown = (e: KeyboardEvent) => {
        if (e.key.toLowerCase() === 'q') {
            splitClip();
        } else if (e.key === 'Delete') {
            mergeClip();
        }
    };

    // 3. Q键分割逻辑
    const splitClip = () => {
        const time = videoRef.current?.currentTime || 0;
        setClips(prev => {
            const index = prev.findIndex(c => time > c.start && time < c.end);
            if (index === -1) return prev;

            const target = prev[index];
            const newClips = [...prev];
            // 将当前片段一分为二
            newClips.splice(index, 1,
                { ...target, id: Date.now(), end: time },
                { ...target, id: Date.now() + 1, start: time }
            );
            return newClips;
        });
    };

    // 4. Delete键合并逻辑
    const mergeClip = () => {
        if (selectedClipId === null) return;
        setClips(prev => {
            const index = prev.findIndex(c => c.id === selectedClipId);
            if (index === -1) return prev;

            const newClips = [...prev];
            if (index === 0) {
                // 最开头的片段：向右合并
                if (newClips.length > 1) {
                    newClips[1].start = newClips[0].start;
                    newClips.splice(0, 1);
                }
            } else {
                // 向左合并：左侧片段继承右侧的结束边界
                newClips[index - 1].end = newClips[index].end;
                newClips.splice(index, 1);
            }
            return newClips;
        });
        setSelectedClipId(null);
    };

    // 5. 双击切换状态
    const toggleStatus = (id: number) => {
        setClips(prev => prev.map(c => {
            if (c.id === id) {
                // Go 传过来的 status 是 string 类型，直接比较即可
                const newStatus = c.status === 'keep' ? 'exclude' : 'keep';
                return { ...c, status: newStatus };
            }
            return c;
        }));
    };

    const handleDownload = async () => {
        // 这里需要将 clips 状态同步给后端 Go 任务对象
        // 实际开发中建议在 Go 增加一个 UpdateClips 方法，这里简化逻辑
        await StartDownload(task.id);
        onClose();
    };

    return (
        <div className="flex flex-col h-full bg-black select-none">
            {/* 顶部标题栏 */}
            <div className="flex justify-between items-center p-4 bg-gray-900 border-b border-gray-800">
                <h2 className="text-sm font-bold truncate mr-4">标记裁切: {task.title}</h2>
                <button onClick={onClose} className="text-gray-400 hover:text-white text-xl">×</button>
            </div>

            {/* 视频预览区 */}
            <div className="flex-1 relative flex items-center justify-center bg-black overflow-hidden">
                <video ref={videoRef} className="max-h-full max-w-full" />
            </div>

            {/* 底部轨道区 */}
            <div className="bg-gray-900 p-6 space-y-4">
                {/* 时间信息 */}
                <div className="flex justify-between text-xs text-gray-400 font-mono">
                    <span>{new Date(currentTime * 1000).toISOString().substr(11, 8)}</span>
                    <span>{new Date(duration * 1000).toISOString().substr(11, 8)}</span>
                </div>

                {/* 核心标记轨道 */}
                <div className="relative h-16 bg-gray-800 rounded-lg overflow-hidden flex border border-gray-700">
                    {clips.map((clip) => (
                        <div
                            key={clip.id}
                            onClick={(e) => {
                                setSelectedClipId(clip.id);
                                if (videoRef.current) {
                                    // 点击跳转到开头，Ctrl+点击跳转到结尾
                                    videoRef.current.currentTime = e.ctrlKey ? clip.end : clip.start;
                                }
                            }}
                            onDoubleClick={() => toggleStatus(clip.id)}
                            style={{ width: `${((clip.end - clip.start) / duration) * 100}%` }}
                            className={`relative h-full border-r border-gray-900 cursor-pointer transition-all flex items-center justify-center
                ${clip.status === 'keep' ? 'bg-blue-600/40 hover:bg-blue-600/60' : 'bg-red-900/40 hover:bg-red-900/60'}
                ${selectedClipId === clip.id ? 'ring-2 ring-white ring-inset z-10' : ''}
              `}
                        >
                            {clip.status === 'exclude' && <span className="text-white font-bold">✕</span>}
                        </div>
                    ))}

                    {/* 播放头游标 */}
                    <div
                        className="absolute top-0 bottom-0 w-0.5 bg-yellow-400 z-20 pointer-events-none"
                        style={{ left: `${(currentTime / duration) * 100}%` }}
                    />
                </div>

                <div className="flex justify-between items-center">
                    <div className="text-[10px] text-gray-500 italic">
                        快捷键: Q 分割 | 双击 切换保留/删除 | Delete 合并左侧 | 点击 跳转
                    </div>
                    <button
                        onClick={handleDownload}
                        className="bg-green-600 hover:bg-green-700 px-8 py-2 rounded-full font-bold transition-all shadow-lg"
                    >
                        开始合并下载
                    </button>
                </div>
            </div>
        </div>
    );
};

export default MarkingPage;