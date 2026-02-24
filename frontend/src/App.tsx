import React, { useEffect, useState } from 'react';
import { MantineProvider, ActionIcon, Tooltip, Text, Button, Group, Badge } from '@mantine/core';
import {
    IconRadar, IconDownload, IconCheck,
    IconBrowser, IconFolder, IconPin, IconX, IconAlertCircle
} from '@tabler/icons-react';

import { EventsOn } from '../wailsjs/runtime';
import {
    GetTasks, StartBrowser, OpenDownloadFolder,
    SetExpanded, TogglePin, QuitApp
} from '../wailsjs/go/main/App';
import { useStore } from './store/useStore';

import SniffedList from './components/SniffedList';
import DownloadList from './components/DownloadList';
import MarkingPage from './components/MarkingPage';

export default function App() {
    const {
        tasks, sniffedMap, activeTargetId, activeTab, isExpanded,
        setTasks, updateTask, addSniffedItem, setActiveTarget,
        removeTab, setTab
    } = useStore();

    const [isPinned, setIsPinned] = useState(true);
    const [showQuitModal, setShowQuitModal] = useState(false);

    useEffect(() => {
        EventsOn("video_sniffed", (item: any) => addSniffedItem(item));
        EventsOn("tab_focused", (tId: string) => setActiveTarget(tId));
        EventsOn("tab_closed", (tId: string) => removeTab(tId));
        EventsOn("task_list_updated", (list: any[]) => setTasks(list));
        EventsOn("task_progress", (task: any) => updateTask(task));
        GetTasks().then(setTasks);
    }, []);

    useEffect(() => { SetExpanded(isExpanded); }, [isExpanded]);

    const sniffCount = activeTargetId ? (sniffedMap[activeTargetId]?.length || 0) : 0;
    const downloadCount = tasks.filter(t => t.status !== 'done' && t.status !== 'error').length;

    const handlePin = async () => setIsPinned(await TogglePin());

    // --- 顶部按钮组件 ---
    const HeaderBtn = ({ icon: Icon, active, onClick, count, title, highlight }: any) => {
        // 浅色模式配色：选中/高亮用 Edge 蓝，未选中用深灰
        const activeColor = '#0078d4';
        const inactiveColor = '#424242';
        const iconColor = active || highlight ? activeColor : inactiveColor;

        return (
            <Tooltip label={title} openDelay={800} bg="white" c="black" withArrow>
                <div style={{
                    position: 'relative',
                    // 关键：按钮区域本身不可拖动，否则无法点击
                    // @ts-ignore
                    "--wails-drop-zone": "no-drag",
                    WebkitAppRegion: "no-drag"
                }}>
                    <ActionIcon
                        variant="subtle" // subtle 模式在浅色下更好看，有 hover 浅灰背景
                        color={active || highlight ? 'blue' : 'gray'}
                        onClick={onClick}
                        size="lg"
                        radius="sm"
                    >
                        <Icon size={20} color={iconColor} stroke={1.5} />
                    </ActionIcon>

                    {/* 红色角标 */}
                    {count > 0 && (
                        <Badge
                            size="xs"
                            circle
                            color="red"
                            style={{
                                position: 'absolute', top: 2, right: 2,
                                width: 14, height: 14, minWidth: 0, padding: 0,
                                fontSize: 9, pointerEvents: 'none', border: '1px solid white'
                            }}
                        >
                            {count > 99 ? '99+' : count}
                        </Badge>
                    )}
                </div>
            </Tooltip>
        );
    };

    return (
        // 1. 强制使用 light 浅色主题
        <MantineProvider defaultColorScheme="light">
            <div style={{ display: 'flex', width: '100vw', height: '100vh', overflow: 'hidden', background: '#ffffff', color: '#333' }}>

                {/* 左侧面板 */}
                <div style={{ width: 380, display: 'flex', flexDirection: 'column', borderRight: '1px solid #e5e5e5' }}>

                    {/* === 顶部 Header === */}
                    <div style={{
                        height: 48, display: 'flex', alignItems: 'center', justifyContent: 'space-between',
                        padding: '0 8px', background: '#f3f3f3', // Edge 浅灰背景
                        borderBottom: '1px solid #e0e0e0',
                        // 2. 关键：整个 Header 设为可拖动
                        // @ts-ignore
                        "--wails-drop-zone": "drag",
                        WebkitAppRegion: "drag",
                        userSelect: 'none'
                    }}>
                        {/* 左侧按钮组 */}
                        <div style={{ display: 'flex', gap: 2 }}>
                            <HeaderBtn icon={IconRadar} title="嗅探列表" active={activeTab==='sniffed'} onClick={()=>setTab('sniffed')} count={sniffCount} />
                            <HeaderBtn icon={IconDownload} title="下载中" active={activeTab==='active'} onClick={()=>setTab('active')} count={downloadCount} />
                            <HeaderBtn icon={IconCheck} title="已完成" active={activeTab==='done'} onClick={()=>setTab('done')} />
                        </div>

                        {/* 右侧按钮组 */}
                        <div style={{ display: 'flex', gap: 2 }}>
                            <HeaderBtn icon={IconBrowser} title="打开浏览器" onClick={()=>StartBrowser()} />
                            <HeaderBtn icon={IconFolder} title="打开文件夹" onClick={()=>OpenDownloadFolder()} />
                            <HeaderBtn icon={IconPin} title={isPinned?"取消置顶":"置顶"} highlight={isPinned} onClick={handlePin} />
                            <HeaderBtn icon={IconX} title="关闭" onClick={() => setShowQuitModal(true)} />
                        </div>
                    </div>

                    {/* 列表内容区 */}
                    <div style={{ flex: 1, overflow: 'hidden', position: 'relative', background: '#ffffff' }}>
                        {activeTab === 'sniffed' && <SniffedList />}
                        {(activeTab === 'active' || activeTab === 'done') && <DownloadList type={activeTab} />}
                    </div>
                </div>

                {/* 右侧扩展区 */}
                {isExpanded && (
                    <div style={{ width: 450, height: '100%', background: '#f9f9f9', borderLeft: '1px solid #d1d1d1' }}>
                        <MarkingPage />
                    </div>
                )}

                {/* 自定义退出确认弹窗 (浅色版) */}
                {showQuitModal && (
                    <div style={{
                        position: 'fixed', inset: 0, zIndex: 9999,
                        backgroundColor: 'rgba(255,255,255,0.8)', // 浅色遮罩
                        backdropFilter: 'blur(2px)',
                        display: 'flex', alignItems: 'center', justifyContent: 'center'
                    }} onClick={(e) => e.stopPropagation()}>
                        <div style={{
                            width: 320, background: '#ffffff', padding: 24, borderRadius: 8,
                            border: '1px solid #e0e0e0', boxShadow: '0 10px 30px rgba(0,0,0,0.1)'
                        }}>
                            <Group gap="xs" mb="sm">
                                <IconAlertCircle color="#d13438" />
                                <Text fw={600} size="lg" c="#333">确认退出程序？</Text>
                            </Group>
                            <Text size="sm" c="dimmed" mb="xl">
                                正在进行的下载任务将会被中断。
                            </Text>
                            <Group justify="flex-end">
                                <Button variant="default" onClick={() => setShowQuitModal(false)}>取消</Button>
                                <Button color="red" onClick={() => QuitApp()}>退出</Button>
                            </Group>
                        </div>
                    </div>
                )}
            </div>
        </MantineProvider>
    );
}