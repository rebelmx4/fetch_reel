import React, { useState, useRef, useEffect } from 'react';
import {
    Box, Stack, Group, Text, Button, ActionIcon,
    Tooltip, Title, Divider, Center, rem
} from '@mantine/core';
import {
    IconX,  IconTrash, IconDownload,
    IconPlayerPlay, IconPlayerPause, IconKeyboard
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

    // 1. 初始化视频
    useEffect(() => {
        if (!markingTask || !videoRef.current) return;

        const video = videoRef.current;
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
            // 默认一个完整的片段
            setClips([{ id: Date.now(), start: 0, end: video.duration, status: 'keep' }]);
        };
        const onTimeUpdate = () => setCurrentTime(video.currentTime);
        const onPlay = () => setIsPlaying(true);
        const onPause = () => setIsPlaying(false);

        video.addEventListener('loadedmetadata', onLoadedMetadata);
        video.addEventListener('timeupdate', onTimeUpdate);
        video.addEventListener('play', onPlay);
        video.addEventListener('pause', onPause);

        // 监听快捷键
        const handleGlobalKeyDown = (e: KeyboardEvent) => {
            if (e.key.toLowerCase() === 'q') splitClip();
            if (e.key === 'Delete') mergeClip();
            if (e.key === ' ') { e.preventDefault(); isPlaying ? video.pause() : video.play(); }
        };
        window.addEventListener('keydown', handleGlobalKeyDown);

        return () => {
            video.removeEventListener('loadedmetadata', onLoadedMetadata);
            video.removeEventListener('timeupdate', onTimeUpdate);
            video.removeEventListener('play', onPlay);
            video.removeEventListener('pause', onPause);
            window.removeEventListener('keydown', handleGlobalKeyDown);
            if (hlsRef.current) hlsRef.current.destroy();
        };
    }, [markingTask]);

    // 2. 核心逻辑：分割
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

    // 3. 核心逻辑：合并 (向左合并)
    const mergeClip = () => {
        if (selectedClipId === null) return;
        setClips(prev => {
            const index = prev.findIndex(c => c.id === selectedClipId);
            if (index <= 0) return prev; // 第一段无法向左合并
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
        // 过滤掉 exclude 的片段，只保留 keep
        const keepClips = clips
            .filter(c => c.status === 'keep')
            .map((c, i) => ({ index: i, start: c.start, end: c.end }));

        // 更新后端的 Clips 信息
        await UpdateTaskClips(markingTask.id, keepClips);
        // 开始下载
        await StartDownload(markingTask.id);

        handleClose();
        setTab('active');
    };

    if (!markingTask) return null;

    return (
        <Stack gap={0} h="100%">
            {/* 顶部栏 */}
            <Group justify="space-between" p="md" bg="dark.7">
                <Text fw={700} size="sm" truncate style={{ maxWidth: rem(300) }}>
                    裁切: {markingTask.title}
                </Text>
                <ActionIcon variant="subtle" color="gray" onClick={handleClose}>
                    <IconX size={20} />
                </ActionIcon>
            </Group>

            <Divider color="dark.5" />

            {/* 视频预览区 (适配 450px 宽度) */}
            <Box bg="black" style={{ flex: 1, position: 'relative' }}>
                <Center h="100%">
                    <video ref={videoRef} style={{ maxWidth: '100%', maxHeight: '100%' }} />
                </Center>
            </Box>

            {/* 底部轨道区 */}
            <Box p="md" bg="dark.7">
                <Stack gap="xs">
                    {/* 时间显示 */}
                    <Group justify="space-between">
                        <Text size="xs" ff="monospace" c="blue.4">
                            {new Date(currentTime * 1000).toISOString().substr(11, 8)}
                        </Text>
                        <Text size="xs" ff="monospace" c="dimmed">
                            {new Date(duration * 1000).toISOString().substr(11, 8)}
                        </Text>
                    </Group>

                    {/* 交互式轨道 */}
                    <Box
                        h={60}
                        bg="dark.6"
                        style={{ borderRadius: rem(8), position: 'relative', overflow: 'hidden', cursor: 'pointer', border: '1px solid #373A40' }}
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
                                        backgroundColor: clip.status === 'keep' ? 'rgba(34, 139, 230, 0.3)' : 'rgba(250, 82, 82, 0.2)',
                                        borderRight: '1px solid rgba(0,0,0,0.5)',
                                        position: 'relative',
                                        transition: 'background 0.2s'
                                    }}
                                    className={selectedClipId === clip.id ? 'selected-clip' : ''}
                                >
                                    {selectedClipId === clip.id && <Box style={{ position: 'absolute', inset: 0, border: '2px solid white', pointerEvents: 'none' }} />}
                                </Box>
                            ))}
                        </Group>

                        {/* 播放指针 */}
                        <Box
                            style={{
                                position: 'absolute',
                                left: `${(currentTime / duration) * 100}%`,
                                top: 0, bottom: 0,
                                width: 2, backgroundColor: '#FCC419', zIndex: 10, pointerEvents: 'none'
                            }}
                        />
                    </Box>

                    {/* 快捷键提示 & 提交 */}
                    <Group justify="space-between" mt="xs">
                        <Group gap={15}>
                            <Stack gap={2}>
                                <Group gap={4}><IconKeyboard size={12}/><Text size="10px">Q 分割</Text></Group>
                                <Group gap={4}><IconTrash size={12}/><Text size="10px">Del 合并左侧</Text></Group>
                            </Stack>
                            <Text size="10px" c="dimmed">双击片段: 保留/排除</Text>
                        </Group>

                        <Button
                            leftSection={<IconDownload size={16} />}
                            color="green"
                            radius="xl"
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