import React from 'react';
import { ScrollArea, Text, Progress, ActionIcon, Menu } from '@mantine/core';
import {
    IconMovie, IconPlayerPlay, IconPlayerPause,
    IconTrash, IconRefresh, IconFile, IconDots
} from '@tabler/icons-react';
import { modals } from '@mantine/modals';
import { useStore } from '../store/useStore';
import { StartDownload, StopDownload, DeleteTask } from '../../wailsjs/go/main/App';

export default function DownloadList({ type }: { type: 'active' | 'done' }) {
    const { tasks, setTab, setRebindingTask } = useStore();

    const list = tasks.filter(t => type === 'done' ? t.status === 'done' : t.status !== 'done');

    const formatSize = (b: number) => {
        if (!b) return '0 B';
        const u = ['B','KB','MB','GB'], i = Math.floor(Math.log(b)/Math.log(1024));
        return parseFloat((b/Math.pow(1024,i)).toFixed(1)) + ' ' + u[i];
    };

    return (
        <ScrollArea h="100%">
            {list.length === 0 && (
                <div style={{padding:40, textAlign:'center', color:'#888', fontSize:13}}>
                    {type==='active' ? '没有正在进行的任务' : '没有已完成的任务'}
                </div>
            )}

            <div style={{ padding: '0' }}>
                {list.map(task => (
                    <div key={task.id}
                         style={{
                             display: 'flex', alignItems: 'center', gap: 12,
                             padding: '12px 14px',
                             borderBottom: '1px solid #f0f0f0',
                             cursor: 'default',
                         }}
                        // 浅色 Hover 效果
                         onMouseEnter={e => e.currentTarget.style.backgroundColor = '#f8f9fa'}
                         onMouseLeave={e => e.currentTarget.style.backgroundColor = 'white'}
                    >
                        <div style={{
                            width: 38, height: 38, background: '#f0f0f0', borderRadius: 6,
                            display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0
                        }}>
                            {task.type === 'mp4' ? <IconMovie size={20} color="#0078d4"/> : <IconFile size={20} color="#666"/>}
                        </div>

                        <div style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', gap: 3 }}>
                            {/* 黑色标题 */}
                            <Text size="sm" c="#222" truncate fw={500}>
                                {task.title || 'Unknown Task'}
                            </Text>

                            {type === 'active' ? (
                                <>
                                    <div style={{display:'flex', justifyContent:'space-between', fontSize: 11, color:'#777'}}>
                                        <span>
                                            {task.status === 'downloading' ? (
                                                <span style={{color:'#0078d4'}}>{task.speed}</span>
                                            ) : (
                                                <span style={{textTransform:'capitalize'}}>{task.status}</span>
                                            )}
                                        </span>
                                        <span>{formatSize(task.downloaded)} / {formatSize(task.size)}</span>
                                    </div>
                                    <Progress
                                        value={task.size > 0 ? (task.downloaded/task.size)*100 : 0}
                                        color={task.status==='error'?'red':'#0078d4'}
                                        size="sm"
                                        radius="xl"
                                        style={{ height: 3, marginTop: 4, background: '#e0e0e0' }}
                                    />
                                </>
                            ) : (
                                <Text size="xs" c="dimmed">
                                    {formatSize(task.size)} • 已完成
                                </Text>
                            )}
                        </div>

                        <div style={{ display: 'flex', gap: 2 }}>
                            {type === 'active' && (
                                <ActionIcon size="md" variant="subtle" color="gray"
                                            onClick={(e) => {
                                                e.stopPropagation();
                                                task.status==='downloading'?StopDownload(task.id):StartDownload(task.id)
                                            }}
                                >
                                    {task.status==='downloading' ? <IconPlayerPause size={18}/> : <IconPlayerPlay size={18}/>}
                                </ActionIcon>
                            )}

                            <Menu shadow="md" width={140} position="bottom-end" withArrow>
                                <Menu.Target>
                                    <ActionIcon size="md" variant="subtle" color="gray">
                                        <IconDots size={18} />
                                    </ActionIcon>
                                </Menu.Target>
                                <Menu.Dropdown>
                                    {type==='active' && (
                                        <Menu.Item leftSection={<IconRefresh size={14}/>} onClick={()=>{setRebindingTask(task);setTab('sniffed')}}>
                                            重新捕获链接
                                        </Menu.Item>
                                    )}
                                    <Menu.Item color="red" leftSection={<IconTrash size={14}/>}
                                               onClick={() => modals.openConfirmModal({
                                                   title:'删除任务', children:'确认删除？文件也将被清理。',
                                                   labels:{confirm:'删除', cancel:'取消'}, confirmProps:{color:'red'},
                                                   onConfirm:()=>DeleteTask(task.id)
                                               })}
                                    >
                                        删除任务
                                    </Menu.Item>
                                </Menu.Dropdown>
                            </Menu>
                        </div>
                    </div>
                ))}
            </div>
        </ScrollArea>
    );
}