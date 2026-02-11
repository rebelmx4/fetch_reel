import React, { useState } from 'react';
import { StartDownload, StopDownload } from '../../wailsjs/go/main/App';
import { BrowserOpenURL } from '../../wailsjs/runtime'; // Wails 内置：打开本地文件夹或网页
import { VideoTask } from '../App';

interface Props {
    tasks: VideoTask[];
}

const DownloadList: React.FC<Props> = ({ tasks }) => {
    const [subTab, setSubTab] = useState<'active' | 'done'>('active');

    // 过滤逻辑
    const activeTasks = tasks.filter(t => t.status !== 'done');
    const completedTasks = tasks.filter(t => t.status === 'done');
    const displayTasks = subTab === 'active' ? activeTasks : completedTasks;

    // 辅助函数：格式化字节大小
    const formatBytes = (bytes: number) => {
        if (bytes === 0) return '0 B';
        const k = 1024;
        const sizes = ['B', 'KB', 'MB', 'GB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
    };

    const handleToggleTask = (task: VideoTask) => {
        if (task.status === 'downloading') {
            StopDownload(task.id);
        } else {
            StartDownload(task.id);
        }
    };

    const openFolder = () => {
        // 假设下载目录在程序运行目录下的 Downloads，这里调用系统打开
        // 在 Go 端我们可以写个专门函数获取，这里先演示用法
        console.log("尝试打开下载目录...");
        // 注意：BrowserOpenURL 在不同系统下行为可能不同，通常传入路径即可
    };

    return (
        <div className="flex flex-col h-full">
            {/* 子标签切换与全局操作 */}
            <div className="flex justify-between items-center px-4 py-2 bg-gray-800/50">
                <div className="flex space-x-4 text-xs">
                    <button
                        onClick={() => setSubTab('active')}
                        className={`pb-1 border-b-2 transition ${subTab === 'active' ? 'border-blue-500 text-blue-400' : 'border-transparent text-gray-500'}`}
                    >
                        进行中 ({activeTasks.length})
                    </button>
                    <button
                        onClick={() => setSubTab('done')}
                        className={`pb-1 border-b-2 transition ${subTab === 'done' ? 'border-blue-500 text-blue-400' : 'border-transparent text-gray-500'}`}
                    >
                        已完成 ({completedTasks.length})
                    </button>
                </div>
                <button
                    onClick={openFolder}
                    className="text-[10px] bg-gray-700 hover:bg-gray-600 px-2 py-1 rounded text-gray-300"
                >
                    打开文件夹
                </button>
            </div>

            {/* 任务列表 */}
            <div className="flex-1 overflow-y-auto">
                {displayTasks.length === 0 && (
                    <div className="text-gray-600 text-center mt-20 text-sm">暂无任务</div>
                )}

                {displayTasks.map((task) => (
                    <div key={task.id} className="p-4 border-b border-gray-800 hover:bg-gray-850">
                        <div className="flex justify-between items-start mb-2">
                            <div className="flex-1 min-w-0">
                                <div className="text-sm font-medium truncate" title={task.title}>{task.title}</div>
                                <div className="text-[10px] text-gray-500 mt-1 uppercase">类型: {task.type}</div>
                            </div>
                            <div className="flex items-center space-x-3">
                <span className={`text-[10px] px-1.5 py-0.5 rounded font-bold uppercase ${
                    task.status === 'downloading' ? 'bg-blue-900 text-blue-300 animate-pulse' :
                        task.status === 'done' ? 'bg-green-900 text-green-300' : 'bg-gray-700 text-gray-400'
                }`}>
                  {task.status}
                </span>
                                {task.status !== 'done' && (
                                    <button
                                        onClick={() => handleToggleTask(task)}
                                        className="text-xs text-blue-400 hover:text-blue-300"
                                    >
                                        {task.status === 'downloading' ? '暂停' : '继续'}
                                    </button>
                                )}
                            </div>
                        </div>

                        {/* 进度条 */}
                        <div className="w-full bg-gray-800 h-1.5 rounded-full overflow-hidden mb-2">
                            <div
                                className="bg-blue-500 h-full transition-all duration-500"
                                style={{ width: `${task.progress}%` }}
                            />
                        </div>

                        {/* 数据统计 */}
                        <div className="flex justify-between text-[10px] text-gray-500 font-mono">
                            <div className="flex space-x-3">
                                <span>已下: {formatBytes(task.downloaded)}</span>
                                {task.status === 'downloading' && (
                                    <span className="text-blue-400 font-bold">速度: {task.speed}</span>
                                )}
                            </div>
                            <span>进度: {task.progress.toFixed(1)}%</span>
                        </div>
                    </div>
                ))}
            </div>
        </div>
    );
};

export default DownloadList;