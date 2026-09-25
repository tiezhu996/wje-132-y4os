import { useEffect, useState } from 'react'
import {
  Button, Card, Checkbox, DatePicker, Descriptions, Form, Input, InputNumber, Modal, Select, Space, Table, Tag, message,
} from 'antd'
import { PlusOutlined } from '@ant-design/icons'
import { createInspection, executeInspection, getInspection } from '@/api/inspection'
import { listRectTasksByInspection, registerRectTask } from '@/api/rectification'
import { listUserOptions } from '@/api/user'
import { useInspectionStore } from '@/stores/inspectionStore'
import { useAuthStore } from '@/stores/authStore'
import StatusBadge from '@/components/common/StatusBadge'
import { formatDate } from '@/utils/dateFormat'
import type { InspectionItem, RectificationTask, SafetyInspection, UserOption } from '@/types'

const typeOptions = [
  { label: '例行检查', value: 'routine' },
  { label: '专项检查', value: 'special' },
  { label: '班前检查', value: 'pre_shift' },
  { label: '应急检查', value: 'emergency' },
]

export default function InspectionManage() {
  const store = useInspectionStore()
  const role = useAuthStore((s) => s.user?.role || '')
  const isManager = role === 'admin' || role === 'safety_manager' || role === 'inspector'
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)
  const [statusFilter, setStatusFilter] = useState('')
  const [open, setOpen] = useState(false)
  const [execId, setExecId] = useState<number>()
  const [items, setItems] = useState<InspectionItem[]>([])
  const [form] = Form.useForm()

  // 详情 / 整改任务登记
  const [detailInspection, setDetailInspection] = useState<SafetyInspection>()
  const [detailItems, setDetailItems] = useState<InspectionItem[]>([])
  const [tasks, setTasks] = useState<RectificationTask[]>([])
  const [users, setUsers] = useState<UserOption[]>([])
  const [regItem, setRegItem] = useState<InspectionItem>()
  const [regAssignee, setRegAssignee] = useState<number>()
  const [regDeadline, setRegDeadline] = useState<any>()

  useEffect(() => {
    store.fetchList({ page, page_size: pageSize, status: statusFilter || undefined })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [page, pageSize, statusFilter])

  useEffect(() => {
    listUserOptions().then((res: any) => setUsers(res.data || []))
  }, [])

  async function onCreate() {
    const values = await form.validateFields()
    await createInspection({
      name: values.name,
      inspection_type: values.inspection_type,
      area: values.area,
      inspection_date: values.inspection_date.format('YYYY-MM-DDTHH:mm:ss'),
      inspector_id: values.inspector_id,
      items: (values.items || []).map((name: string) => ({ item_name: name })),
    })
    message.success('检查计划创建成功')
    setOpen(false)
    form.resetFields()
    setPage(1)
  }

  // 执行检查：勾出不合格项 + 备注；执行完可在详情中为不合格项登记整改任务
  async function openExecute(id: number) {
    const res: any = await getInspection(id)
    setItems(res.data.items)
    setExecId(id)
  }

  async function onExecute() {
    if (!execId) return
    await executeInspection(execId, items)
    message.success('检查执行完成，可在详情中为不合格项登记整改任务')
    setExecId(undefined)
    store.fetchList({ page, page_size: pageSize, status: statusFilter || undefined })
    openDetail(execId)
  }

  async function openDetail(id: number) {
    const [insRes, taskRes]: any[] = await Promise.all([getInspection(id), listRectTasksByInspection(id)])
    setDetailInspection(insRes.data.inspection)
    setDetailItems(insRes.data.items)
    setTasks(taskRes.data || [])
  }

  function closeDetail() {
    setDetailInspection(undefined)
    setDetailItems([])
    setTasks([])
    setRegItem(undefined)
  }

  function openRegister(item: InspectionItem) {
    setRegItem(item)
    setRegAssignee(undefined)
    setRegDeadline(undefined)
  }

  async function onRegister() {
    if (!detailInspection || !regItem) return
    if (!regAssignee) {
      message.warning('请选择责任人')
      return
    }
    if (!regDeadline) {
      message.warning('请选择整改期限')
      return
    }
    await registerRectTask({
      inspection_id: detailInspection.id,
      item_id: regItem.id,
      assignee_id: regAssignee,
      deadline: regDeadline.format('YYYY-MM-DDTHH:mm:ss'),
    })
    message.success('整改任务已登记，可在「整改跟踪」中办理')
    setRegItem(undefined)
    const taskRes: any = await listRectTasksByInspection(detailInspection.id)
    setTasks(taskRes.data || [])
  }

  const taskByItem = new Map(tasks.map((t) => [t.item_id, t]))

  return (
    <Card>
      <Space style={{ marginBottom: 16 }}>
        <Select placeholder="状态" allowClear style={{ width: 130 }} options={[
          { label: '待执行', value: 'scheduled' },
          { label: '执行中', value: 'in_progress' },
          { label: '已完成', value: 'completed' },
          { label: '有不合格项', value: 'failed' },
        ]} onChange={(v) => { setStatusFilter(v || ''); setPage(1) }} />
        <Button type="primary" icon={<PlusOutlined />} onClick={() => setOpen(true)}>创建检查计划</Button>
      </Space>
      <Table<SafetyInspection>
        rowKey="id"
        dataSource={store.list}
        pagination={{ current: page, pageSize, total: store.total, onChange: (p, ps) => { setPage(p); setPageSize(ps) } }}
        columns={[
          { title: 'ID', dataIndex: 'id', width: 70 },
          { title: '名称', dataIndex: 'name' },
          { title: '类型', dataIndex: 'inspection_type', render: (v) => typeOptions.find((o) => o.value === v)?.label || v },
          { title: '区域', dataIndex: 'area' },
          { title: '检查日期', dataIndex: 'inspection_date', render: (v) => formatDate(v) },
          { title: '得分', dataIndex: 'total_score', render: (v) => `${v} 分` },
          {
            title: '不合格项',
            dataIndex: 'issue_count',
            width: 90,
            render: (v: number) => (v > 0 ? <Tag color="red">{v} 项</Tag> : <Tag>0 项</Tag>),
          },
          { title: '状态', dataIndex: 'status', render: (v) => <StatusBadge status={v} /> },
          {
            title: '操作',
            render: (_, row) =>
              row.status === 'scheduled' || row.status === 'in_progress' ? (
                <Button size="small" type="primary" onClick={() => openExecute(row.id)}>执行检查</Button>
              ) : (
                <a onClick={() => openDetail(row.id)}>查看详情</a>
              ),
          },
        ]}
      />
      <Modal title="创建检查计划" open={open} onOk={onCreate} onCancel={() => setOpen(false)} width={560}>
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="检查名称" rules={[{ required: true }]}><Input /></Form.Item>
          <Form.Item name="inspection_type" label="检查类型" rules={[{ required: true }]}><Select options={typeOptions} /></Form.Item>
          <Form.Item name="area" label="区域"><Input /></Form.Item>
          <Form.Item name="inspection_date" label="检查日期" rules={[{ required: true }]}><DatePicker showTime style={{ width: '100%' }} /></Form.Item>
          <Form.Item name="inspector_id" label="检查人ID" rules={[{ required: true }]}><InputNumber style={{ width: '100%' }} /></Form.Item>
          <Form.Item name="items" label="检查项（每行一个）">
            <Select mode="tags" placeholder="输入后回车添加检查项" open={false} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal title="执行检查" open={!!execId} onOk={onExecute} onCancel={() => setExecId(undefined)} width={600}>
        <div style={{ marginBottom: 8, color: '#999' }}>勾选表示合格，不勾选即为不合格项；不合格项请填写问题备注，执行后可登记整改任务。</div>
        {items.map((item) => (
          <div key={item.id || item.item_name} style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 8 }}>
            <Checkbox checked={item.passed} onChange={(e) => setItems((prev) => prev.map((x) => (x.id === item.id ? { ...x, passed: e.target.checked } : x)))}>
              {item.item_name}
            </Checkbox>
            <Input
              placeholder={item.passed ? '备注' : '不合格问题描述'}
              value={item.remark}
              status={!item.passed ? 'warning' : undefined}
              onChange={(e) => setItems((prev) => prev.map((x) => (x.id === item.id ? { ...x, remark: e.target.value } : x)))}
            />
          </div>
        ))}
      </Modal>

      <Modal
        title={detailInspection ? `检查详情：${detailInspection.name}` : '检查详情'}
        open={!!detailInspection}
        footer={null}
        width={720}
        onCancel={closeDetail}
      >
        {detailInspection && (
          <Space direction="vertical" style={{ width: '100%' }} size={16}>
            <Descriptions column={3} size="small">
              <Descriptions.Item label="区域">{detailInspection.area || '-'}</Descriptions.Item>
              <Descriptions.Item label="检查日期">{formatDate(detailInspection.inspection_date)}</Descriptions.Item>
              <Descriptions.Item label="得分">{detailInspection.total_score} 分</Descriptions.Item>
            </Descriptions>
            <Table<InspectionItem>
              rowKey="id"
              size="small"
              pagination={false}
              dataSource={detailItems}
              columns={[
                { title: '检查项', dataIndex: 'item_name' },
                {
                  title: '结果',
                  dataIndex: 'passed',
                  width: 90,
                  render: (v: boolean) => (v ? <Tag color="green">合格</Tag> : <Tag color="red">不合格</Tag>),
                },
                { title: '备注', dataIndex: 'remark', render: (v: string) => v || '-' },
                {
                  title: '整改任务',
                  width: 240,
                  render: (_, row) => {
                    if (row.passed) return '-'
                    const task = taskByItem.get(row.id)
                    if (!task) {
                      return isManager ? (
                        <Button size="small" type="link" onClick={() => openRegister(row)}>登记整改任务</Button>
                      ) : (
                        <Tag>未登记</Tag>
                      )
                    }
                    const overdue = task.status !== 'approved' && new Date(task.deadline).getTime() < Date.now()
                    return (
                      <Space size={4} wrap>
                        <StatusBadge status={task.status} />
                        {overdue && <Tag color="error">已逾期</Tag>}
                        <span style={{ color: '#999', fontSize: 12 }}>期限 {formatDate(task.deadline)}</span>
                      </Space>
                    )
                  },
                },
              ]}
            />
          </Space>
        )}
      </Modal>

      {/* 登记整改任务 */}
      <Modal title="登记整改任务" open={!!regItem} onOk={onRegister} onCancel={() => setRegItem(undefined)} width={480}>
        {regItem && (
          <Space direction="vertical" style={{ width: '100%' }} size={12}>
            <Descriptions column={1} size="small" bordered>
              <Descriptions.Item label="不合格项">{regItem.item_name}</Descriptions.Item>
              <Descriptions.Item label="问题描述">{regItem.remark || '-'}</Descriptions.Item>
            </Descriptions>
            <div>
              <div style={{ marginBottom: 4 }}>责任人</div>
              <Select
                style={{ width: '100%' }}
                placeholder="选择整改责任人"
                value={regAssignee}
                onChange={setRegAssignee}
                options={users.map((u) => ({ value: u.id, label: u.name || u.role }))}
                optionFilterProp="label"
                showSearch
              />
            </div>
            <div>
              <div style={{ marginBottom: 4 }}>整改期限</div>
              <DatePicker showTime style={{ width: '100%' }} value={regDeadline} onChange={setRegDeadline} />
            </div>
          </Space>
        )}
      </Modal>
    </Card>
  )
}
