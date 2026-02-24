import React, { useState } from 'react';
import { ScrollArea, Text, ActionIcon, Collapse, Box, Badge } from '@mantine/core';
import { IconMovie, IconPlayerPlay, IconPlayerStop, IconScissors, IconDownload } from '@tabler/icons-react';
import { useStore } from '../store/useStore';
import { CreateDownloadTask, StartDownload } from '../../wailsjs/go/main/App';

export default function SniffedList() {
    const { sniffedMap, activeTargetId, setExpanded, setMarkingTask, setTab } = useStore();
    const [previewIdx, setPreviewIdx] = useState<number | null>(null);

    const items = activeTargetId ? (sniffedMap[activeTargetId] || []) : [];

    // 文件名处理
    const getCleanName = (urlStr: string) => {
        try {
            const u = new URL(urlStr);
            const name = u.pathname.split('/').pop();
            return name && name.trim() !== '' ? name : 'video.mp4';
        } catch { return 'video_resource'; }
    };

    const handleMark = async (item: any) => {
        const task = await CreateDownloadTask(item);
        setMarkingTask(task);
        setExpanded(true);
    };

    const handleDL = async (item: any) => {
        const task = await CreateDownloadTask(item);
        await StartDownload(task.id);
        setTab('active');
    };

    if (!activeTargetId) return (
        <div style={{height: '100%', display:'flex', alignItems:'center', justifyContent:'center', color:'#888', fontSize:13}}>
            <div>请在浏览器中打开包含视频的标签页</div>
        </div>
    );

    return (
        <ScrollArea h="100%" scrollbars="y">
            <div style={{ padding: '0' }}>
                {items.length === 0 && (
                    <div style={{padding:40, textAlign:'center', color:'#888', fontSize:13}}>
                        当前页面未检测到视频
                    </div>
                )}

                {items.map((item, idx) => {
                    const displayName = getCleanName(item.url);
                    const isPreviewing = previewIdx === idx;

                    return (
                        <div key={idx} style={{
                            padding: '10px 14px',
                            borderBottom: '1px solid #f0f0f0', // 浅色分割线
                            backgroundColor: isPreviewing ? '#f8f9fa' : 'white',
                            transition: 'background 0.2s'
                        }}
                             onMouseEnter={(e)=>{if(!isPreviewing) e.currentTarget.style.background = '#f5f5f5'}}
                             onMouseLeave={(e)=>{if(!isPreviewing) e.currentTarget.style.background = 'white'}}
                        >
                            <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
                                {/* 图标背景改为浅灰 */}
                                <div style={{
                                    width: 36, height: 36, borderRadius: 6, background: '#f0f0f0',
                                    display: 'flex', alignItems: 'center', justifyContent: 'center'
                                }}>
                                    <IconMovie size={20} color="#555" />
                                </div>

                                <div style={{ flex: 1, minWidth: 0 }}>
                                    {/* 标题颜色改为深灰/黑 */}
                                    <Text size="sm" c="#222" truncate fw={500}>
                                        {displayName}
                                    </Text>
                                    <div style={{display:'flex', alignItems:'center', gap: 8, marginTop:2}}>
                                        <Badge size="xs" color="gray" radius="sm" variant="light">
                                            {item.type}
                                        </Badge>
                                        <Text size="xs" c="dimmed">
                                            {item.size > 0 ? (item.size/1024/1024).toFixed(1) + ' MB' : '未知大小'}
                                        </Text>
                                    </div>
                                </div>

                                <div style={{ display: 'flex', gap: 4 }}>
                                    <ActionIcon variant="subtle" color="gray" onClick={()=>setPreviewIdx(isPreviewing?null:idx)}>
                                        {isPreviewing ? <IconPlayerStop size={18}/> : <IconPlayerPlay size={18}/>}
                                    </ActionIcon>
                                    <ActionIcon variant="subtle" color="blue" onClick={()=>handleMark(item)}>
                                        <IconScissors size={18}/>
                                    </ActionIcon>
                                    <ActionIcon variant="subtle" color="teal" onClick={()=>handleDL(item)}>
                                        <IconDownload size={18}/>
                                    </ActionIcon>
                                </div>
                            </div>

                            <Collapse in={isPreviewing}>
                                <Box mt={10} style={{borderRadius: 6, overflow:'hidden', border: '1px solid #eee'}}>
                                    {isPreviewing && (
                                        <video controls autoPlay style={{width:'100%', display:'block', maxHeight: 240, background: 'black'}}
                                               src={`http://127.0.0.1:12345/proxy?url=${encodeURIComponent(item.url)}&referer=${encodeURIComponent(item.originUrl)}`}
                                        />
                                    )}
                                </Box>
                            </Collapse>
                        </div>
                    );
                })}
            </div>
        </ScrollArea>
    );
}