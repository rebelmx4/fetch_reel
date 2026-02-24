import React, { useState, useRef, useEffect } from 'react';
import {
    Box, Stack, Group, Text, Button, ActionIcon,
    Divider, Center, rem, Tooltip
} from '@mantine/core';
import {
    IconX, IconTrash, IconDownload,
    IconPlayerPlay, IconPlayerPause, IconKeyboard, IconScissors
} from '@tabler/icons-react';
import Hls from 'hls.js';
import { useStore } from '../store/useStore';
import { StartDownload, UpdateTaskClips } from '../../wailsjs/go/main/App';

export default function MarkingPage() {
    const { markingTask, setMarkingTask, setExpanded, setTab } = useStore();
    const [duration, setDuration] = useState(0);
    const [currentTime, setCurrentTime] = useState(0);
    const [clips, setClips] = useState<any[]>([]);
    const [selectedClipId, setSelectedClipId] = useState<number | null>(null);
    const [isPlaying, setIsPlaying] = useState(false);

    const videoRef = useRef<HTMLVideoElement>(null);
    const hlsRef = useRef<Hls | null>(null);

    // 1. 视频初始化逻辑
    useEffect(() => {
        if (!markingTask || !videoRef.current) return;

        const video = videoRef.current;
        // 使用后端 proxy 服务
        const proxyUrl = `http://127.0.0.1:12345/proxy?url=${encodeURIComponent(markingTask.url)}&referer=${encodeURIComponent(markingTask.originUrl)}`;

        if (markingTask.type === 'hls' && Hls.isSupported()) {
            const hls = new Hls();
            hls.loadSource(proxyUrl);
            hls.attachMedia(video);
            hlsRef.current = hls;
        } else {
            video.src = proxyUrl;
        }

        const onLoadedMetadata = () => {
            setDuration(video.duration);
            // 初始化一段完整的区间
            setClips([{ id: Date.now(), start: 0, end: video.duration, status: 'keep' }]);
        };
        const onTimeUpdate = () => setCurrentTime(video.currentTime);
        const onPlay = () => setIsPlaying(true);
        const onPause = () => setIsPlaying(false);

        video.addEventListener('loadedmetadata', onLoadedMetadata);
        video.addEventListener('timeupdate', onTimeUpdate);
        video.addEventListener('play', onPlay);
        video.addEventListener('pause', onPause);

        const handleKeyDown = (e: KeyboardEvent) => {
            if (e.key.toLowerCase() === 'q') splitClip();
            if (e.key === 'Delete') mergeClip();
            if (e.key === ' ') { e.preventDefault(); video.paused ? video.play() : video.pause(); }
        };
        window.addEventListener('keydown', handleKeyDown);

        return () => {
            video.removeEventListener('loadedmetadata', onLoadedMetadata);
            video.removeEventListener('timeupdate', onTimeUpdate);
            video.removeEventListener('play', onPlay);
            video.removeEventListener('pause', onPause);
            window.removeEventListener('keydown', handleKeyDown);
            if (hlsRef.current) hlsRef.current.destroy();
        };
    }, [markingTask]);

    const splitClip = () => {
        const time = videoRef.current?.currentTime || 0;
        setClips(prev => {
            const index = prev.findIndex(c => time > c.start && time < c.end);
            if (index === -1) return prev;
            const target = prev[index];
            const newClips = [...prev];
            newClips.splice(index, 1,
                { ...target, id: Date.now(), end: time },
                { ...target, id: Date.now() + 1, start: time }
            );
            return newClips;
        });
    };

    const mergeClip = () => {
        if (selectedClipId === null) return;
        setClips(prev => {
            const index = prev.findIndex(c => c.id === selectedClipId);
            if (index <= 0) return prev;
            const newClips = [...prev];
            newClips[index - 1].end = newClips[index].end;
            newClips.splice(index, 1);
            return newClips;
        });
        setSelectedClipId(null);
    };

    const handleClose = () => {
        setExpanded(false);
        setMarkingTask(null);
    };

    const handleDownload = async () => {
        if (!markingTask) return;
        const keepClips = clips
            .filter(c => c.status === 'keep')
            .map((c, i) => ({ index: i, start: c.start, end: c.end }));

        await UpdateTaskClips(markingTask.id, keepClips);
        await StartDownload(markingTask.id);

        handleClose();
        setTab('active');
    };

    if (!markingTask) return null;

    return (
        <Stack gap={0} h="100%" bg="#1A1B1E">
            {/* 顶部标题栏 */}
            <Group justify="space-between" p="sm" bg="#25262B">
                <Group gap="xs">
                    <IconScissors size={18} color="#228be6" />
                    <Text fw={700} size="sm">视频裁切标记</Text>
                </Group>
                <ActionIcon variant="subtle" color="gray" onClick={handleClose}>
                    <IconX size={20} />
                </ActionIcon>
            </Group>

            <Divider color="#373A40" />

            {/* 视频预览区 */}
            <Box bg="black" style={{ flex: 1, position: 'relative', overflow: 'hidden' }}>
                <Center h="100%">
                    <video ref={videoRef} style={{ maxWidth: '100%', maxHeight: '100%' }} />
                </Center>
                {!isPlaying && (
                    <Box style={{ position: 'absolute', inset: 0, display: 'flex', alignItems: 'center', justifyContent: 'center', pointerEvents: 'none' }}>
                        <IconPlayerPlay size={48} color="white" style={{ opacity: 0.3 }} />
                    </Box>
                )}
            </Box>

            {/* 交互控制区 */}
            <Box p="md" bg="#25262B">
                <Stack gap="xs">
                    <Group justify="space-between">
                        <Text size="xs" ff="monospace" c="blue.4">
                            {new Date(currentTime * 1000).toISOString().substr(11, 8)}
                        </Text>
                        <Text size="xs" ff="monospace" c="dimmed">
                            {new Date(duration * 1000).toISOString().substr(11, 8)}
                        </Text>
                    </Group>

                    {/* 裁切轨道 */}
                    <Box
                        h={50}
                        bg="#141517"
                        style={{ borderRadius: rem(4), position: 'relative', overflow: 'hidden', cursor: 'pointer', border: '1px solid #373A40' }}
                    >
                        <Group gap={0} h="100%" wrap="nowrap">
                            {clips.map((clip) => (
                                <Box
                                    key={clip.id}
                                    onClick={(e) => {
                                        setSelectedClipId(clip.id);
                                        if (videoRef.current) videoRef.current.currentTime = e.ctrlKey ? clip.end : clip.start;
                                    }}
                                    onDoubleClick={() => {
                                        setClips(prev => prev.map(c => c.id === clip.id ? { ...c, status: c.status === 'keep' ? 'exclude' : 'keep' } : c));
                                    }}
                                    style={{
                                        width: `${((clip.end - clip.start) / duration) * 100}%`,
                                        height: '100%',
                                        backgroundColor: clip.status === 'keep' ? 'rgba(34, 139, 230, 0.4)' : 'rgba(250, 82, 82, 0.15)',
                                        borderRight: '1px solid rgba(255,255,255,0.1)',
                                        position: 'relative',
                                        boxSizing: 'border-box'
                                    }}
                                >
                                    {selectedClipId === clip.id && <Box style={{ position: 'absolute', inset: 0, border: '1.5px solid white', pointerEvents: 'none' }} />}
                                </Box>
                            ))}
                        </Group>
                        {/* 播放头 */}
                        <Box style={{
                            position: 'absolute',
                            left: `${(currentTime / duration) * 100}%`,
                            top: 0, bottom: 0, width: 2, backgroundColor: '#FCC419', zIndex: 10, pointerEvents: 'none'
                        }} />
                    </Box>

                    <Group justify="space-between" mt="xs">
                        <Stack gap={2}>
                            <Group gap={4}>
                                <IconKeyboard size={14} color="gray" />
                                <Text size="10px" c="dimmed">Q 分割 / Space 播放</Text>
                            </Group>

                            <Group gap={4}>
                                <IconTrash size={14} color="gray" /> 
                                <Text size="10px" c="dimmed">Del 合并左侧 / 双击 排除</Text>
                            </Group></Stack>
                        <Button
                            leftSection={<IconDownload size={18} />}
                            color="blue"
                            variant="filled"
                            onClick={handleDownload}
                        >
                            开始合并下载
                        </Button>
                    </Group>
                </Stack>
            </Box>
        </Stack>
    );
}