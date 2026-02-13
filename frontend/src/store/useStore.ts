import { create } from 'zustand';
import { engine } from '../../wailsjs/go/models';

// 定义 Zustand 存储的状态结构
interface TaskState {
    // 核心数据
    tasks: engine.VideoTask[];
    // 嗅探到的资源：Key 是 targetId (Chrome 标签页 ID)
    sniffedMap: Record<string, engine.SniffEvent[]>;
    // 当前 Chrome 正在看的标签页 ID
    activeTargetId: string | null;

    // UI 状态
    activeTab: 'sniffed' | 'active' | 'done';
    isExpanded: boolean; // 是否处于裁切扩宽模式 (800px)

    // Actions
    setTasks: (tasks: engine.VideoTask[]) => void;
    updateTask: (task: engine.VideoTask) => void;
    addSniffedItem: (item: engine.SniffEvent) => void;
    setActiveTarget: (targetId: string) => void;
    removeTab: (targetId: string) => void;

    setTab: (tab: 'sniffed' | 'active' | 'done') => void;
    setExpanded: (expanded: boolean) => void;

    markingTask: engine.VideoTask | null; // 正在被裁切的任务
    setMarkingTask: (task: engine.VideoTask | null) => void;

    rebindingTask: engine.VideoTask | null; // 正在等待重绑的任务
    setRebindingTask: (task: engine.VideoTask | null) => void;
}

export const useStore = create<TaskState>((set) => ({
    tasks: [],
    sniffedMap: {},
    activeTargetId: null,
    activeTab: 'sniffed',
    isExpanded: false,
    markingTask: null,
    rebindingTask: null,

    setTasks: (tasks) => set({ tasks }),

    updateTask: (updatedTask) => set((state) => ({
        tasks: state.tasks.map(t => t.id === updatedTask.id ? updatedTask : t)
    })),

    setMarkingTask: (task) => set({ markingTask: task }),
    setRebindingTask: (task: engine.VideoTask | null) => set({ rebindingTask: task }),

    addSniffedItem: (item) => set((state) => {
        const targetId = item.targetId;
        const existing = state.sniffedMap[targetId] || [];

        // URL 去重
        if (existing.some(i => i.url === item.url)) return state;

        return {
            sniffedMap: {
                ...state.sniffedMap,
                [targetId]: [item, ...existing]
            }
        };
    }),

    setActiveTarget: (targetId) => set({ activeTargetId: targetId }),

    removeTab: (targetId) => set((state) => {
        const newMap = { ...state.sniffedMap };
        delete newMap[targetId];
        return { sniffedMap: newMap };
    }),

    setTab: (tab) => set({ activeTab: tab }),

    setExpanded: (expanded) => set({ isExpanded: expanded }),
}));