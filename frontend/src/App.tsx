import React, { useState, useEffect } from 'react';
import { EventsOn } from '../wailsjs/runtime'; // Wails 提供的事件监听
import { StartBrowser, GetTasks } from '../wailsjs/go/main/App';
import SniffedList from './components/SniffedList';
import DownloadList from './components/DownloadList';
import MarkingPage from './components/MarkingPage';

import { engine } from '../wailsjs/go/models'; // 导入 Go 生成的模型

// 使用类型别名，这样你就不需要修改后面代码里的变量名了
export type Clip = engine.Clip;
export type VideoTask = engine.VideoTask;
export type SniffEvent = engine.SniffEvent;
// debugger;

function App() {
    const [activeTab, setActiveTab] = useState<'sniffed' | 'downloads'>('sniffed');
    const [sniffedItems, setSniffedItems] = useState<SniffEvent[]>([]);
    const [tasks, setTasks] = useState<VideoTask[]>([]);
    const [markingTask, setMarkingTask] = useState<VideoTask | null>(null);

    useEffect(() => {
        // 1. 监听后端嗅探到的视频事件
        EventsOn("video_sniffed", (item: SniffEvent) => {
            setSniffedItems(prev => {
                // 简单去重：如果 URL 已存在则不添加
                if (prev.find(i => i.url === item.url)) return prev;
                return [item, ...prev];
            });
        });

        // 2. 监听任务列表更新（持久化加载或新任务添加）
        EventsOn("task_list_updated", (updatedTasks: VideoTask[]) => {
            setTasks(updatedTasks);
        });

        // 3. 监听下载进度更新
        EventsOn("task_progress", (updatedTask: VideoTask) => {
            setTasks(prev => prev.map(t => t.id === updatedTask.id ? updatedTask : t));
        });

        // 初始化时获取一次现有任务
        GetTasks().then(setTasks);
    }, []);

    const handleStartBrowser = () => {
        StartBrowser();
    };

    return (
        <div className="flex flex-col h-screen bg-gray-900 text-white overflow-hidden">
            {/* 顶部操作区 */}
            <div className="p-4 border-b border-gray-700 flex justify-between items-center">
                <button
                    onClick={handleStartBrowser}
                    className="bg-blue-600 hover:bg-blue-700 px-4 py-2 rounded text-sm font-bold transition"
                >
                    打开浏览器
                </button>
                <div className="flex space-x-2">
                    <button
                        onClick={() => setActiveTab('sniffed')}
                        className={`px-3 py-1 rounded ${activeTab === 'sniffed' ? 'bg-gray-700' : 'text-gray-400'}`}
                    >
                        嗅探 ({sniffedItems.length})
                    </button>
                    <button
                        onClick={() => setActiveTab('downloads')}
                        className={`px-3 py-1 rounded ${activeTab === 'downloads' ? 'bg-gray-700' : 'text-gray-400'}`}
                    >
                        下载 ({tasks.length})
                    </button>
                </div>
            </div>

            {/* 主体列表区 */}
            <div className="flex-1 overflow-y-auto">
                {activeTab === 'sniffed' ? (
                    <SniffedList items={sniffedItems} onMark={setMarkingTask} />
                ) : (
                    <DownloadList tasks={tasks} />
                )}
            </div>

            {/* 全屏遮罩：裁切标记页面 */}
            {markingTask && (
                <div className="fixed inset-0 z-50 bg-black">
                    <MarkingPage
                        task={markingTask}
                        onClose={() => setMarkingTask(null)}
                    />
                </div>
            )}
        </div>
    );
}

export default App;