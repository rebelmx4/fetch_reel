import React, { useState, useEffect, useRef } from 'react';
import {
    ScrollArea, Card, Text, Group, Badge, ActionIcon,
    Button, Stack, Collapse, Box, Center, Tooltip // 确保这里有 Collapse 和 Tooltip
} from '@mantine/core';

import {
    IconPlayerPlay,
    IconScissors, // 修正拼写：增加 s
    IconDownload,
    IconMovie
} from '@tabler/icons-react';
import Hls from 'hls.js';
import { useStore } from '../store/useStore';
import { CreateDownloadTask, StartDownload } from '../../wailsjs/go/main/App';

export default function SniffedList() {
    const { sniffedMap, activeTargetId, setExpanded, setMarkingTask } = useStore();
    const [previewIndex, setPreviewIndex] = useState<number | null>(null);
    const videoRef = useRef<HTMLVideoElement>(null);

    // 只获取当前活跃标签页的资源
    const items = activeTargetId ? (sniffedMap[activeTargetId] || []) : [];

    // 处理预览播放
    useEffect(() => {
        if (previewIndex !== null && videoRef.current) {
            const item = items[previewIndex];
            const proxyUrl = `http://127.0.0.1:12345/proxy?url=${encodeURIComponent(item.url)}&referer=${encodeURIComponent(item.originUrl)}`;

            if (item.type === 'hls' && Hls.isSupported()) {
                const hls = new Hls();
                hls.loadSource(proxyUrl);
                hls.attachMedia(videoRef.current);
            } else {
                videoRef.current.src = proxyUrl;
            }
        }
    }, [previewIndex, items]);

    const handleMark = async (item: any) => {
        // 1. 调用后端创建任务，获取分配好的 ID 和临时目录
        const task = await CreateDownloadTask(item);
        // 2. 将任务存入 Store 供 MarkingPage 使用
        setMarkingTask(task);
        // 3. 扩宽窗口并展示裁切页
        setExpanded(true);
    };

    const handleDownload = async (item: any) => {
        const task = await CreateDownloadTask(item);
        await StartDownload(task.id);
        // 自动切换到“正在下载”标签
        useStore.getState().setTab('active');
    };

    // 辅助函数：截取 URL 最后一段
    const getShortUrl = (url: string) => {
        try {
            const parts = url.split('/');
            const last = parts.pop() || parts.pop(); // 处理末尾带斜杠的情况
            return last?.split('?')[0] || "video_stream";
        } catch {
            return "video_stream";
        }
    };

    if (!activeTargetId) {
        return (
            <Center h="80vh" p="xl">
                <Text c="dimmed" size="sm" ta="center">请在 Chrome 中选择一个标签页进行嗅探</Text>
            </Center>
        );
    }

    return (
        <ScrollArea h="calc(100vh - 50px)" p="xs">
            {items.length === 0 ? (
                <Center h="50vh">
                    <Stack align="center" gap="xs">
                        <IconMovie size={40} color="#373A40" />
                        <Text c="dimmed" size="xs">当前页面暂无媒体资源</Text>
                    </Stack>
                </Center>
            ) : (
                <Stack gap="sm">
                    {items.map((item, index) => (
                        <Card key={index} withBorder padding="sm" radius="md">
                            <Stack gap="xs">
                                {/* 标题与基本信息 */}
                                <div>
                                    <Text size="sm" fw={700} truncate title={item.title}>
                                        {item.title || "未知视频"}
                                    </Text>
                                    <Text size="10px" c="dimmed" truncate mt={2}>
                                        {getShortUrl(item.url)}
                                    </Text>
                                </div>

                                <Group justify="space-between">
                                    <Group gap={5}>
                                        <Badge size="xs" variant="light" color={item.type === 'hls' ? 'orange' : 'blue'}>
                                            {item.type}
                                        </Badge>
                                        {item.size > 0 && (
                                            <Badge size="xs" variant="outline">
                                                {(item.size / 1024 / 1024).toFixed(1)}MB
                                            </Badge>
                                        )}
                                    </Group>

                                    {/* 操作按钮组 */}
                                    <Group gap={5}>
                                        <Tooltip label="预览">
                                            <ActionIcon
                                                variant={previewIndex === index ? "filled" : "light"}
                                                onClick={() => setPreviewIndex(previewIndex === index ? null : index)}
                                            >
                                                <IconPlayerPlay size={16} />
                                            </ActionIcon>
                                        </Tooltip>
                                        <Tooltip label="裁切标记">
                                            <ActionIcon variant="light" color="grape" onClick={() => handleMark(item)}>
                                                <IconScissors size={16} />
                                            </ActionIcon>
                                        </Tooltip>
                                        <Tooltip label="全量下载">
                                            <ActionIcon variant="light" color="green" onClick={() => handleDownload(item)}>
                                                <IconDownload size={16} />
                                            </ActionIcon>
                                        </Tooltip>
                                    </Group>
                                </Group>

                                {/* 预览折叠区 */}
                                <Collapse in={previewIndex === index}>
                                    <Box mt="xs" bg="black" style={{ borderRadius: '4px', overflow: 'hidden' }}>
                                        <video
                                            ref={previewIndex === index ? videoRef : null}
                                            controls
                                            autoPlay
                                            style={{ width: '100%', display: 'block' }}
                                        />
                                    </Box>
                                </Collapse>
                            </Stack>
                        </Card>
                    ))}
                </Stack>
            )}
        </ScrollArea>
    );
}