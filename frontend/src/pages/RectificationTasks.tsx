import { useEffect, useState } from 'react'
import { Card, DatePicker, Descriptions, Drawer, Form, Input, InputNumber, Modal, Radio, Select, Space, Table, Timeline, message } from 'antd'
import { useSearchParams } from 'react-router-dom'
import {
  assignRectificationTask,
  getRectificationTask,
  reviewRectificationTask,
  submitRectificationTask,
} from '@/api/rectification'
import { useRectificationStore } from '@/stores/rectificationStore'
import { useAuthStore } from '@/stores/authStore'
import RectificationStatusBadge from '@/components/common/RectificationStatusBadge'
import RoleGuard from '@/components/common/RoleGuard'
import EmptyState from '@/components/common/EmptyState'
import { RectificationActionText, RectificationFilterOptions, RectificationStatusOptions } from '@/constants/rectification'
import { formatDateTime } from '@/utils/dateFormat'
import type { RectificationRecord, RectificationTask } from '@/types'

export default function RectificationTasks() {
  const store = useRectificationStore()
  const me = useAuthStore((s) => s.user)
  const [searchParams] = useSearchParams()
  const inspectionId = searchParams.get('inspection_id') || undefined
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)
  const [filter, setFilter] = useState(searchParams.get('filter') || '')
  const [statusFilter, setStatusFilter] = useState('')
  const [detailId, setDetailId] = useState<number>()
  const [detail, setDetail] = useState<{ task: RectificationTask; records: RectificationRecord[] }>()
  const [assignTask, setAssignTask] = useState<RectificationTask>()
  const [submitTask, setSubmitTask] = useState<RectificationTask>()
  const [reviewTask, setReviewTask] = useState<RectificationTask>()
  const [assignForm] = Form.useForm()
  const [submitForm] = Form.useForm()
  const [reviewForm] = Form.useForm()

  const query = () => ({
    page,
    page_size: pageSize,
    filter: filter || undefined,
    status: statusFilter || undefined,
    inspection_id: inspectionId,
  })

  useEffect(() => {
    store.fetchList(query())
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [page, pageSize, filter, statusFilter, inspectionId])

  useEffect(() => {
    if (!detailId) {
      setDetail(undefined)
      return
    }
    getRectificationTask(detailId).then((res: any) => setDetail(res.data))
  }, [detailId])

  async function refresh() {
    await store.fetchList(query())
    if (detailId) {
      const res: any = await getRectificationTask(detailId)
      setDetail(res.data)
    }
  }

  async function onAssign() {
    if (!assignTask) return
    const values = await assignForm.validateFields()
    await assignRectificationTask(assignTask.id, {
      assignee_id: values.assignee_id,
      deadline: values.deadline.format('YYYY-MM-DDTHH:mm:ss'),
    })
    message.success('整改任务已登记责任人与期限')
    setAssignTask(undefined)
    assignForm.resetFields()
    refresh()
  }

  async function onSubmit() {
    if (!submitTask) return
    const values = await submitForm.validateFields()
    await submitRectificationTask(submitTask.id, { note: values.note })
    message.success('整改说明已提交，等待复查')
    setSubmitTask(undefined)
    submitForm.resetFields()
    refresh()
  }

  async function onReview() {
    if (!reviewTask) return
    const values = await reviewForm.validateFields()
    await reviewRectificationTask(reviewTask.id, { passed: values.passed, note: values.note })
    message.success(values.passed ? '复查通过，任务已闭环' : '已退回整改')
    setReviewTask(undefined)
    reviewForm.resetFields()
    refresh()
  }

  function canSubmit(row: RectificationTask) {
    if (row.status !== 'pending' && row.status !== 'returned') return false
    if (!row.assignee_id) return false
    return me?.id === row.assignee_id || me?.role === 'admin'
  }

  return (
    <Card title={inspectionId ? `整改任务（检查 #${inspectionId}）` : '整改任务'}>
      <Space style={{ marginBottom: 16 }}>
        <Select
          placeholder="快捷筛选"
          allowClear
          style={{ width: 130 }}
          value={filter || undefined}
          options={RectificationFilterOptions}
          onChange={(v) => { setFilter(v || ''); setPage(1) }}
        />
        <Select
          placeholder="状态"
          allowClear
          style={{ width: 130 }}
          options={RectificationStatusOptions}
          onChange={(v) => { setStatusFilter(v || ''); setPage(1) }}
        />
      </Space>
      <Table<RectificationTask>
        rowKey="id"
        dataSource={store.list}
        locale={{ emptyText: <EmptyState text="暂无整改任务" /> }}
        rowClassName={(row) => (row.overdue ? 'rectification-row-overdue' : '')}
        pagination={{ current: page, pageSize, total: store.total, onChange: (p, ps) => { setPage(p); setPageSize(ps) } }}
        columns={[
          { title: 'ID', dataIndex: 'id', width: 70 },
          { title: '不合格项', dataIndex: 'item_name' },
          { title: '所属检查', dataIndex: 'inspection_name' },
          { title: '责任人', dataIndex: 'assignee_name', render: (v, row) => v || (row.assignee_id ? `#${row.assignee_id}` : '未登记') },
          { title: '整改期限', dataIndex: 'deadline', render: (v) => (v ? formatDateTime(v) : '未登记') },
          { title: '状态', dataIndex: 'status', render: (v, row) => <RectificationStatusBadge status={v} overdue={row.overdue} /> },
          {
            title: '操作',
            width: 240,
            render: (_, row) => (
              <Space size={4} wrap>
                <a onClick={() => setDetailId(row.id)}>详情</a>
                <RoleGuard roles={['admin', 'safety_manager', 'inspector']}>
                  {(row.status === 'pending' || row.status === 'returned') && (
                    <a onClick={() => setAssignTask(row)}>登记</a>
                  )}
                  {row.status === 'submitted' && <a onClick={() => setReviewTask(row)}>复查</a>}
                </RoleGuard>
                {canSubmit(row) && <a onClick={() => setSubmitTask(row)}>提交整改</a>}
              </Space>
            ),
          },
        ]}
      />
      <Drawer title={`整改任务 #${detailId} 办理记录`} open={!!detailId} onClose={() => setDetailId(undefined)} width={480}>
        {detail && (
          <>
            <Descriptions column={1} size="small" bordered>
              <Descriptions.Item label="不合格项">{detail.task.item_name}</Descriptions.Item>
              <Descriptions.Item label="所属检查">{detail.task.inspection_name || `#${detail.task.inspection_id}`}</Descriptions.Item>
              <Descriptions.Item label="责任人">{detail.task.assignee_name || '未登记'}</Descriptions.Item>
              <Descriptions.Item label="整改期限">{detail.task.deadline ? formatDateTime(detail.task.deadline) : '未登记'}</Descriptions.Item>
              <Descriptions.Item label="状态">
                <RectificationStatusBadge status={detail.task.status} overdue={detail.task.overdue} />
              </Descriptions.Item>
            </Descriptions>
            <h4 style={{ margin: '16px 0' }}>办理记录</h4>
            <Timeline
              items={detail.records.map((r) => ({
                color: r.action === 'review_return' ? 'red' : r.action === 'review_pass' ? 'green' : 'blue',
                children: (
                  <>
                    <div>
                      <b>{RectificationActionText[r.action] || r.action}</b>
                      <span style={{ color: '#999', marginLeft: 8 }}>{r.operator_name || `#${r.operator_id}`} · {formatDateTime(r.created_at)}</span>
                    </div>
                    {r.content && <div style={{ color: '#666' }}>{r.content}</div>}
                  </>
                ),
              }))}
            />
          </>
        )}
      </Drawer>
      <Modal title={`登记整改任务：${assignTask?.item_name || ''}`} open={!!assignTask} onOk={onAssign} onCancel={() => setAssignTask(undefined)} width={480}>
        <Form form={assignForm} layout="vertical">
          <Form.Item name="assignee_id" label="责任人ID" rules={[{ required: true, message: '请填写责任人ID' }]}>
            <InputNumber style={{ width: '100%' }} min={1} placeholder="整改责任人的用户ID" />
          </Form.Item>
          <Form.Item name="deadline" label="整改期限" rules={[{ required: true, message: '请选择整改期限' }]}>
            <DatePicker showTime style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      </Modal>
      <Modal title={`提交整改说明：${submitTask?.item_name || ''}`} open={!!submitTask} onOk={onSubmit} onCancel={() => setSubmitTask(undefined)} width={480}>
        <Form form={submitForm} layout="vertical">
          <Form.Item name="note" label="整改说明" rules={[{ required: true, message: '请填写整改说明' }]}>
            <Input.TextArea rows={4} maxLength={500} placeholder="说明已完成的整改措施，提交后等待复查" />
          </Form.Item>
        </Form>
      </Modal>
      <Modal title={`复查：${reviewTask?.item_name || ''}`} open={!!reviewTask} onOk={onReview} onCancel={() => setReviewTask(undefined)} width={480}>
        <Form form={reviewForm} layout="vertical" initialValues={{ passed: true }}>
          <Form.Item name="passed" label="复查结论" rules={[{ required: true }]}>
            <Radio.Group
              options={[
                { label: '复查通过', value: true },
                { label: '退回整改', value: false },
              ]}
              optionType="button"
            />
          </Form.Item>
          <Form.Item
            noStyle
            shouldUpdate={(prev, cur) => prev.passed !== cur.passed}
          >
            {({ getFieldValue }) => (
              <Form.Item
                name="note"
                label={getFieldValue('passed') ? '复查意见' : '退回原因'}
                rules={[{ required: !getFieldValue('passed'), message: '退回必须填写退回原因' }]}
              >
                <Input.TextArea rows={3} maxLength={500} placeholder={getFieldValue('passed') ? '可填写复查意见' : '说明不通过的原因，原整改记录将保留'} />
              </Form.Item>
            )}
          </Form.Item>
        </Form>
      </Modal>
    </Card>
  )
}
