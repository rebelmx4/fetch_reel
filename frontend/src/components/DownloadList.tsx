import React from 'react';
import {
    ScrollArea, Card, Text, Group, Progress, ActionIcon,
    Stack, Badge, Tooltip, Menu, Center, Box, rem
} from '@mantine/core';
import {
    IconPlayerPause, IconPlayerPlay, IconTrash, IconRefresh,
    IconExternalLink, IconAlertTriangle, IconCheck, IconDownload
} from '@tabler/icons-react';
import { modals } from '@mantine/modals'; // 需要在 App.tsx 外层包裹 ModalsProvider
import { useStore } from '../store/useStore';
import { StartDownload, StopDownload, DeleteTask, UpdateTaskUrl } from '../../wailsjs/go/main/App';

interface Props {
    type: 'active' | 'done';
}

export default function DownloadList({ type }: Props) {
    const { tasks, setTab, setRebindingTask } = useStore();

    // 1. 过滤任务
    const displayTasks = tasks.filter(t => {
        if (type === 'done') return t.status === 'done';
        return t.status !== 'done';
    });

    // 2. 格式化工具
    const formatBytes = (bytes: number) => {
        if (bytes <= 0) return '0 B';
        const i = Math.floor(Math.log(bytes) / Math.log(1024));
        return (bytes / Math.pow(1024, i)).toFixed(1) + ' ' + ['B', 'KB', 'MB', 'GB'][i];
    };

    const getFileName = (path: string) => {
        const parts = path.split(/[\\/]/);
        return parts[parts.length - 1];
    };

    // 3. 操作处理
    const handleToggle = (task: any) => {
        if (task.status === 'downloading') {
            StopDownload(task.id);
        } else {
            StartDownload(task.id);
        }
    };

    const handleDelete = (task: any) => {
        modals.openConfirmModal({
            title: '确认删除任务',
            centered: true,
            children: (
                <Text size="sm">
                    你确定要删除任务 "{getFileName(task.savePath)}" 吗？该操作不可撤销。
                </Text>
            ),
            labels: { confirm: '删除', cancel: '取消' },
            confirmProps: { color: 'red' },
            onConfirm: () => DeleteTask(task.id),
        });
    };

    // 4. 重绑定逻辑 (Re-bind)
    const handleRebind = (task: any) => {
        setRebindingTask(task); // 进入重绑定模式
        setTab('sniffed');      // 自动跳转到嗅探页
    };

    return (
        <ScrollArea h="calc(100vh - 50px)" p="xs">
            {displayTasks.length === 0 ? (
                <Center h="50vh">
                    <Text c="dimmed" size="xs">暂无{type === 'done' ? '已完成' : '下载中'}的任务</Text>
                </Center>
            ) : (
                <Stack gap="sm">
                    {displayTasks.map((task) => (
                        <Card key={task.id} withBorder padding="xs" radius="md">
                            <Stack gap={5}>
                                {/* 标题行 */}
                                <Group justify="space-between" wrap="nowrap">
                                    <Text size="xs" fw={700} truncate flex={1} title={getFileName(task.savePath)}>
                                        {getFileName(task.savePath)}
                                    </Text>

                                    {/* 操作按钮 */}
                                    <Group gap={4}>
                                        {type === 'active' && (
                                            <>
                                                <ActionIcon
                                                    variant="subtle"
                                                    size="sm"
                                                    color={task.status === 'downloading' ? 'blue' : 'green'}
                                                    onClick={() => handleToggle(task)}
                                                >
                                                    {task.status === 'downloading' ? <IconPlayerPause size={14} /> : <IconPlayerPlay size={14} />}
                                                </ActionIcon>
                                                <Tooltip label="重新嗅探并绑定链接">
                                                    <ActionIcon variant="subtle" size="sm" color="orange" onClick={() => handleRebind(task)}>
                                                        <IconRefresh size={14} />
                                                    </ActionIcon>
                                                </Tooltip>
                                            </>
                                        )}
                                        <ActionIcon variant="subtle" size="sm" color="red" onClick={() => handleDelete(task)}>
                                            <IconTrash size={14} />
                                        </ActionIcon>
                                    </Group>
                                </Group>

                                {/* 进度条与速度 (仅下载中展示) */}
                                {type === 'active' && (
                                    <>
                                        <Progress
                                            value={task.progress}
                                            size="xs"
                                            color={task.status === 'error' ? 'red' : 'blue'}
                                            animated={task.status === 'downloading'}
                                        />
                                        <Group justify="space-between">
                                            <Group gap={8}>
                                                <Text size="10px" c="dimmed">
                                                    {formatBytes(task.size > 0 ? (task.size * task.progress / 100) : 0)} / {task.size > 0 ? formatBytes(task.size) : '未知'}
                                                </Text>
                                                {task.status === 'downloading' && (
                                                    <Text size="10px" c="blue" fw={700}>{task.speed}</Text>
                                                )}
                                            </Group>
                                            <Text size="10px" c="dimmed">{task.progress.toFixed(1)}%</Text>
                                        </Group>
                                    </>
                                )}

                                {/* 完成状态信息 */}
                                {type === 'done' && (
                                    <Group justify="space-between">
                                        <Text size="10px" c="dimmed">{formatBytes(task.size)}</Text>
                                        <Badge size="xs" color="green" variant="light" leftSection={<IconCheck size={10} />}>
                                            已完成
                                        </Badge>
                                    </Group>
                                )}

                                {/* 错误状态提醒 */}
                                {task.status === 'error' && (
                                    <Group gap={4}>
                                        <IconAlertTriangle size={12} color="red" />
                                        <Text size="10px" color="red">下载失败，请尝试重新嗅探链接</Text>
                                    </Group>
                                )}
                            </Stack>
                        </Card>
                    ))}
                </Stack>
            )}
        </ScrollArea>
    );
}