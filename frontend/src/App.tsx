import React, { useEffect } from 'react';
import {
    MantineProvider, AppShell, Tooltip, UnstyledButton,
    Stack, Group, ActionIcon, Title, Badge, rem // 确保导入了 rem
} from '@mantine/core';

import { IconBrowser, IconFolder, IconRadar, IconDownload, IconCheck, IconScissors } from '@tabler/icons-react';
import { EventsOn } from '../wailsjs/runtime';
import { GetTasks, StartBrowser, OpenDownloadFolder, SetExpanded } from '../wailsjs/go/main/App';
import { useStore } from './store/useStore';
import { ModalsProvider } from '@mantine/modals';

// 导入子组件 (稍后提供)
import SniffedList from './components/SniffedList';
import DownloadList from './components/DownloadList';
import MarkingPage from './components/MarkingPage';

// 侧边栏按钮组件
interface NavbarLinkProps {
    icon: typeof IconRadar;
    label: string;
    active?: boolean;
    onClick?(): void;
    count?: number;
}

function NavbarLink({ icon: Icon, label, active, onClick, count }: NavbarLinkProps) {
    return (
        <Tooltip label={label} position="right" transitionProps={{ duration: 0 }}>
            <UnstyledButton
                onClick={onClick}
                data-active={active || undefined}
                style={(theme) => ({
                    width: rem(50),
                    height: rem(50),
                    borderRadius: theme.radius.md,
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    color: active ? theme.white : theme.colors.dark[0],
                    backgroundColor: active ? theme.colors.blue[9] : 'transparent',
                    '&:hover': {
                        backgroundColor: theme.colors.dark[6],
                    },
                })}
            >
                <div style={{ position: 'relative' }}>
                    <Icon style={{ width: rem(24), height: rem(24) }} stroke={1.5} />
                    {count !== undefined && count > 0 && (
                        <Badge
                            size="xs"
                            variant="filled"
                            color="red"
                            circle
                            style={{ position: 'absolute', top: -5, right: -10 }}
                        >
                            {count}
                        </Badge>
                    )}
                </div>
            </UnstyledButton>
        </Tooltip>
    );
}

export default function App() {
    const {
        tasks, sniffedMap, activeTargetId, activeTab, isExpanded,
        setTasks, updateTask, addSniffedItem, setActiveTarget, removeTab, setTab, setExpanded
    } = useStore();

    // 1. 注册后端事件监听
    useEffect(() => {
        EventsOn("video_sniffed", (item: any) => addSniffedItem(item));
        EventsOn("tab_focused", (targetId: string) => setActiveTarget(targetId));
        EventsOn("tab_closed", (targetId: string) => removeTab(targetId));
        EventsOn("task_list_updated", (updatedTasks: any[]) => setTasks(updatedTasks));
        EventsOn("task_progress", (updatedTask: any) => updateTask(updatedTask));

        // 初始化加载任务
        GetTasks().then(setTasks);
    }, []);

    // 2. 监听裁切模式切换窗口宽度
    useEffect(() => {
        SetExpanded(isExpanded);
    }, [isExpanded]);

    // 计算当前标签页的资源数量
    const currentSniffCount = activeTargetId ? (sniffedMap[activeTargetId]?.length || 0) : 0;
    const activeTaskCount = tasks.filter(t => t.status !== 'done').length;

    return (
        <MantineProvider defaultColorScheme="dark">
            <ModalsProvider>
            <AppShell
                padding="0"
                header={{ height: 50 }}
                navbar={{ width: 65, breakpoint: 'sm' }}
            >
                {/* 顶部工具栏 */}
                <AppShell.Header p="xs" style={{ borderBottom: '1px solid #373A40' }}>
                    <Group justify="space-between" h="100%">
                        <Title order={6} c="blue.4">FetchReel</Title>
                        <Group gap="xs">
                            <Tooltip label="启动浏览器">
                                <ActionIcon variant="light" size="lg" onClick={() => StartBrowser()}>
                                    <IconBrowser size={20} />
                                </ActionIcon>
                            </Tooltip>
                            <Tooltip label="打开下载目录">
                                <ActionIcon variant="light" size="lg" onClick={() => OpenDownloadFolder()}>
                                    <IconFolder size={20} />
                                </ActionIcon>
                            </Tooltip>
                        </Group>
                    </Group>
                </AppShell.Header>

                {/* 左侧紧凑导航栏 */}
                <AppShell.Navbar p="xs">
                    <Stack justify="center" gap={10}>
                        <NavbarLink
                            icon={IconRadar}
                            label="嗅探资源"
                            active={activeTab === 'sniffed'}
                            onClick={() => setTab('sniffed')}
                            count={currentSniffCount}
                        />
                        <NavbarLink
                            icon={IconDownload}
                            label="正在下载"
                            active={activeTab === 'active'}
                            onClick={() => setTab('active')}
                            count={activeTaskCount}
                        />
                        <NavbarLink
                            icon={IconCheck}
                            label="已完成"
                            active={activeTab === 'done'}
                            onClick={() => setTab('done')}
                        />
                    </Stack>
                </AppShell.Navbar>

                {/* 主体内容区 */}
                <AppShell.Main style={{ display: 'flex', flexDirection: 'row' }}>
                    {/* 列表区 (始终占据 350px 宽度) */}
                    <div style={{ width: '100%', height: '100%', overflow: 'hidden' }}>
                        {activeTab === 'sniffed' && <SniffedList />}
                        {(activeTab === 'active' || activeTab === 'done') && <DownloadList type={activeTab} />}
                    </div>

                    {/* 裁切扩展区 (当 isExpanded 为 true 时展示) */}
                    {isExpanded && (
                        <div style={{
                            width: 450,
                            borderLeft: '1px solid #373A40',
                            backgroundColor: '#1A1B1E',
                            height: '100vh',
                            position: 'fixed',
                            left: 350,
                            top: 0,
                            zIndex: 100
                        }}>
                            <MarkingPage />
                        </div>
                    )}
                </AppShell.Main>
            </AppShell>
            </ModalsProvider>
        </MantineProvider>
    );
}