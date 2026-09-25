import { useEffect, useMemo, useState } from 'react'
import {
  Button,
  Card,
  DatePicker,
  Descriptions,
  Form,
  Image,
  Input,
  Modal,
  Select,
  Space,
  Table,
  Tag,
  Timeline,
  Upload,
  message,
} from 'antd'
import { PlusOutlined, UploadOutlined } from '@ant-design/icons'
import { useSearchParams } from 'react-router-dom'
import {
  getRectTask,
  listRectTasks,
  registerRectTask,
  reviewRectTask,
  submitRectTask,
} from '@/api/rectification'
import { getInspection, listInspections } from '@/api/inspection'
import { listUserOptions } from '@/api/user'
import { useAuthStore } from '@/stores/authStore'
import { useFileUpload } from '@/hooks/useFileUpload'
import StatusBadge from '@/components/common/StatusBadge'
import { formatDate, formatDateTime } from '@/utils/dateFormat'
import { RectHistoryActionColor, RectHistoryActionText, RectTaskStatus } from '@/constants/rectification'
import type { RectificationHistory, RectificationTask, UserOption } from '@/types'

const filterOptions = [
  { label: '待整改', value: 'pending_rectification' },
  { label: '已逾期', value: 'overdue' },
  { label: '待复查', value: RectTaskStatus.SUBMITTED },
  { label: '已退回', value: RectTaskStatus.REJECTED },
  { label: '复查通过', value: RectTaskStatus.APPROVED },
  { label: '全部', value: 'all' },
]

// 可登记整改任务的不合格检查项（来自检查详情接口）
interface IssueItem {
  inspection_id: number
  inspection_name: string
  item_id: number
  item_name: string
  remark: string
}

export default function RectificationTasks() {
  const role = useAuthStore((s) => s.user?.role || '')
  const myId = useAuthStore((s) => s.user?.id || 0)
  const isManager = role === 'admin' || role === 'safety_manager' || role === 'inspector'

  const [list, setList] = useState<RectificationTask[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)
  const [searchParams] = useSearchParams()
  const initialStatus = searchParams.get('status') || 'pending_rectification'
  const [statusFilter, setStatusFilter] = useState<string>(
    filterOptions.some((o) => o.value === initialStatus) ? initialStatus : 'pending_rectification',
  )
  const [mineOnly, setMineOnly] = useState(false)
  const [users, setUsers] = useState<UserOption[]>([])

  const [detail, setDetail] = useState<RectificationTask>()
  const [histories, setHistories] = useState<RectificationHistory[]>([])
  const [registerOpen, setRegisterOpen] = useState(false)
  const [registerIssues, setRegisterIssues] = useState<IssueItem[]>([])
  const [regForm] = Form.useForm()
  const [submitTarget, setSubmitTarget] = useState<RectificationTask>()
  const [submitNote, setSubmitNote] = useState('')
  const [submitPhoto, setSubmitPhoto] = useState('')
  const [reviewTarget, setReviewTarget] = useState<RectificationTask>()
  const [reviewApproved, setReviewApproved] = useState(true)
  const [reviewNote, setReviewNote] = useState('')
  const { uploading, upload } = useFileUpload()

  const userName = useMemo(() => {
    const m = new Map<number, string>()
    users.forEach((u) => m.set(u.id, u.name || u.role))
    return (id?: number) => (id ? m.get(id) || `#${id}` : '-')
  }, [users])

  async function fetchList() {
    const res: any = await listRectTasks({
      page,
      page_size: pageSize,
      status: !statusFilter || statusFilter === 'all' ? undefined : statusFilter,
      mine: mineOnly ? 1 : undefined,
    })
    setList(res.data.list)
    setTotal(res.data.total)
  }

  useEffect(() => {
    fetchList()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [page, pageSize, statusFilter, mineOnly])

  useEffect(() => {
    listUserOptions().then((res: any) => setUsers(res.data || []))
  }, [])

  async function openDetail(row: RectificationTask) {
    const res: any = await getRectTask(row.id)
    setDetail(res.data.task)
    setHistories(res.data.histories || [])
  }

  // 拉取全部检查，找出尚未登记任务的不合格检查项
  async function openRegister() {
    const [taskRes, insListRes]: any[] = await Promise.all([
      listRectTasks({ page: 1, page_size: 500 }),
      listInspections({ page: 1, page_size: 200, status: 'failed' }),
    ])
    const registeredItemIDs = new Set<number>((taskRes.data.list as RectificationTask[]).map((t) => t.item_id))
    const inspections = insListRes.data.list as { id: number; name: string }[]
    const issues: IssueItem[] = []
    await Promise.all(
      inspections.map(async (ins) => {
        const detailRes: any = await getInspection(ins.id)
        ;(detailRes.data.items as { id: number; item_name: string; passed: boolean; remark: string }[]).forEach((it) => {
          if (!it.passed && !registeredItemIDs.has(it.id)) {
            issues.push({ inspection_id: ins.id, inspection_name: ins.name, item_id: it.id, item_name: it.item_name, remark: it.remark })
          }
        })
      }),
    )
    issues.sort((a, b) => a.inspection_id - b.inspection_id || a.item_id - b.item_id)
    if (issues.length === 0) {
      message.info('没有待登记的不合格项')
      return
    }
    setRegisterIssues(issues)
    regForm.resetFields()
    setRegisterOpen(true)
  }

  async function onRegister() {
    const values = await regForm.validateFields()
    const issue = registerIssues.find((it) => it.item_id === values.item_id)
    if (!issue) {
      message.warning('请选择不合格检查项')
      return
    }
    await registerRectTask({
      inspection_id: issue.inspection_id,
      item_id: issue.item_id,
      assignee_id: values.assignee_id,
      deadline: values.deadline.format('YYYY-MM-DDTHH:mm:ss'),
    })
    message.success('整改任务已登记')
    setRegisterOpen(false)
    setPage(1)
    setStatusFilter('pending_rectification')
    fetchList()
  }

  function openSubmit(row: RectificationTask) {
    setSubmitTarget(row)
    setSubmitNote(row.status === RectTaskStatus.REJECTED ? row.rectification_note : '')
    setSubmitPhoto(row.status === RectTaskStatus.REJECTED ? row.rectification_photo : '')
  }

  async function onSubmit() {
    if (!submitTarget) return
    if (!submitNote.trim()) {
      message.warning('请填写整改说明')
      return
    }
    await submitRectTask(submitTarget.id, { rectification_note: submitNote, rectification_photo: submitPhoto })
    message.success('整改说明已提交，等待复查')
    setSubmitTarget(undefined)
    fetchList()
  }

  function openReview(row: RectificationTask, approved: boolean) {
    setReviewTarget(row)
    setReviewApproved(approved)
    setReviewNote('')
  }

  async function onReview() {
    if (!reviewTarget) return
    if (!reviewApproved && !reviewNote.trim()) {
      message.warning('退回时必须填写复查意见')
      return
    }
    await reviewRectTask(reviewTarget.id, { approved: reviewApproved, review_note: reviewNote })
    message.success(reviewApproved ? '复查通过' : '已退回责任人重新整改')
    setReviewTarget(undefined)
    fetchList()
  }

  const columns = [
    { title: 'ID', dataIndex: 'id', width: 70 },
    {
      title: '不合格项',
      render: (_: unknown, row: RectificationTask) => (
        <div>
          <div style={{ fontWeight: 600 }}>{row.item_name}</div>
          <div style={{ color: '#999', fontSize: 12 }}>
            {row.inspection_name}
            {row.area ? ` · ${row.area}` : ''}
          </div>
        </div>
      ),
    },
    { title: '责任人', dataIndex: 'assignee_name', width: 100, render: (v: string) => v || '-' },
    {
      title: '整改期限',
      dataIndex: 'deadline',
      width: 120,
      render: (v: string, row: RectificationTask) => (
        <Space direction="vertical" size={0}>
          <span>{formatDate(v)}</span>
          {row.overdue && <Tag color="error">已逾期</Tag>}
        </Space>
      ),
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 120,
      render: (v: string, row: RectificationTask) => (
        <Space size={4} direction="vertical">
          <StatusBadge status={v} />
          {row.overdue && <Tag color="error" style={{ marginInlineEnd: 0 }}>已逾期</Tag>}
        </Space>
      ),
    },
    {
      title: '操作',
      width: 230,
      render: (_: unknown, row: RectificationTask) => {
        const mine = row.assignee_id === myId
        const canSubmit = mine && (row.status === RectTaskStatus.PENDING || row.status === RectTaskStatus.REJECTED)
        const canReview = isManager && row.status === RectTaskStatus.SUBMITTED
        return (
          <Space size={4}>
            <a onClick={() => openDetail(row)}>详情</a>
            {canSubmit && (
              <a onClick={() => openSubmit(row)}>{row.status === RectTaskStatus.REJECTED ? '重新提交' : '提交整改'}</a>
            )}
            {canReview && (
              <>
                <a style={{ color: '#52c41a' }} onClick={() => openReview(row, true)}>复查通过</a>
                <a style={{ color: '#cf1322' }} onClick={() => openReview(row, false)}>退回</a>
              </>
            )}
          </Space>
        )
      },
    },
  ]

  return (
    <Card
      title="整改任务"
      extra={
        isManager && (
          <Button type="primary" icon={<PlusOutlined />} onClick={openRegister}>
            登记整改任务
          </Button>
        )
      }
    >
      <Space style={{ marginBottom: 16 }} wrap>
        <Select
          style={{ width: 150 }}
          value={statusFilter}
          options={filterOptions}
          onChange={(v) => {
            setStatusFilter(v || '')
            setPage(1)
          }}
        />
        <Select
          style={{ width: 140 }}
          value={mineOnly ? 'mine' : 'all'}
          options={[
            { label: '全部任务', value: 'all' },
            { label: '我负责的', value: 'mine' },
          ]}
          onChange={(v) => {
            setMineOnly(v === 'mine')
            setPage(1)
          }}
        />
      </Space>
      <Table
        rowKey="id"
        dataSource={list}
        pagination={{ current: page, pageSize, total, onChange: (p, ps) => { setPage(p); setPageSize(ps) } }}
        columns={columns}
        rowClassName={(row) => (row.overdue ? 'rect-task-overdue' : '')}
      />

      {/* 登记整改任务 */}
      <Modal title="登记整改任务" open={registerOpen} onOk={onRegister} onCancel={() => setRegisterOpen(false)} width={520}>
        <Form form={regForm} layout="vertical">
          <Form.Item name="item_id" label="不合格检查项" rules={[{ required: true, message: '请选择不合格检查项' }]}>
            <Select
              placeholder="选择检查中勾出的不合格项"
              options={registerIssues.map((it) => ({
                value: it.item_id,
                label: `${it.item_name}（${it.inspection_name}${it.remark ? '：' + it.remark : ''}）`,
              }))}
              optionFilterProp="label"
              showSearch
            />
          </Form.Item>
          <Form.Item name="assignee_id" label="责任人" rules={[{ required: true, message: '请选择责任人' }]}>
            <Select
              placeholder="选择整改责任人"
              options={users.map((u) => ({ value: u.id, label: u.name || u.role }))}
              optionFilterProp="label"
              showSearch
            />
          </Form.Item>
          <Form.Item name="deadline" label="整改期限" rules={[{ required: true, message: '请选择整改期限' }]}>
            <DatePicker showTime style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      </Modal>

      {/* 提交整改说明 */}
      <Modal
        title={submitTarget?.status === RectTaskStatus.REJECTED ? '重新提交整改' : '提交整改说明'}
        open={!!submitTarget}
        onOk={onSubmit}
        onCancel={() => setSubmitTarget(undefined)}
        width={560}
      >
        {submitTarget && (
          <Space direction="vertical" style={{ width: '100%' }} size={12}>
            <Descriptions column={1} size="small" bordered>
              <Descriptions.Item label="不合格项">{submitTarget.item_name}</Descriptions.Item>
              <Descriptions.Item label="整改期限">{formatDate(submitTarget.deadline)}</Descriptions.Item>
              {submitTarget.status === RectTaskStatus.REJECTED && (
                <Descriptions.Item label="上次退回意见">
                  <span style={{ color: '#cf1322' }}>{submitTarget.review_note || '-'}</span>
                </Descriptions.Item>
              )}
            </Descriptions>
            <Input.TextArea
              rows={4}
              maxLength={500}
              showCount
              placeholder="填写整改措施与整改情况"
              value={submitNote}
              onChange={(e) => setSubmitNote(e.target.value)}
            />
            <Upload
              showUploadList={false}
              beforeUpload={(file) => {
                upload(file).then((url) => setSubmitPhoto(url))
                return false
              }}
            >
              <Button icon={<UploadOutlined />} loading={uploading}>上传整改照片</Button>
            </Upload>
            {submitPhoto && <Image src={submitPhoto} alt="整改照片" width={160} />}
          </Space>
        )}
      </Modal>

      {/* 复查 */}
      <Modal
        title={`复查整改任务 #${reviewTarget?.id ?? ''}`}
        open={!!reviewTarget}
        onOk={onReview}
        onCancel={() => setReviewTarget(undefined)}
        okText={reviewApproved ? '确认复查通过' : '确认退回'}
        okButtonProps={{ danger: !reviewApproved }}
        width={560}
      >
        {reviewTarget && (
          <Space direction="vertical" style={{ width: '100%' }} size={12}>
            <Descriptions column={1} size="small" bordered>
              <Descriptions.Item label="不合格项">{reviewTarget.item_name}</Descriptions.Item>
              <Descriptions.Item label="责任人">{userName(reviewTarget.assignee_id)}</Descriptions.Item>
              <Descriptions.Item label="整改说明">{reviewTarget.rectification_note || '-'}</Descriptions.Item>
              <Descriptions.Item label="提交时间">{formatDateTime(reviewTarget.submitted_at)}</Descriptions.Item>
              {reviewTarget.rectification_photo && (
                <Descriptions.Item label="整改照片">
                  <Image src={reviewTarget.rectification_photo} width={180} />
                </Descriptions.Item>
              )}
            </Descriptions>
            <div>
              <Space style={{ marginBottom: 8 }}>
                <Button type={reviewApproved ? 'primary' : 'default'} onClick={() => setReviewApproved(true)}>复查通过</Button>
                <Button danger={!reviewApproved} type={!reviewApproved ? 'primary' : 'default'} onClick={() => setReviewApproved(false)}>退回重新整改</Button>
              </Space>
              {!reviewApproved && (
                <Input.TextArea
                  rows={3}
                  maxLength={500}
                  showCount
                  placeholder="请填写退回意见（必填），原提交记录将保留"
                  value={reviewNote}
                  onChange={(e) => setReviewNote(e.target.value)}
                />
              )}
            </div>
          </Space>
        )}
      </Modal>

      {/* 详情 / 流转记录 */}
      <Modal title={`整改任务详情 #${detail?.id ?? ''}`} open={!!detail} footer={null} onCancel={() => setDetail(undefined)} width={640}>
        {detail && (
          <Space direction="vertical" style={{ width: '100%' }} size={16}>
            <Descriptions column={2} bordered size="small">
              <Descriptions.Item label="不合格项" span={2}>{detail.item_name}</Descriptions.Item>
              <Descriptions.Item label="责任人">{userName(detail.assignee_id)}</Descriptions.Item>
              <Descriptions.Item label="整改期限">
                {formatDate(detail.deadline)}
                {detail.overdue && <Tag color="error" style={{ marginLeft: 8 }}>已逾期</Tag>}
              </Descriptions.Item>
              <Descriptions.Item label="状态">
                <StatusBadge status={detail.status} />
              </Descriptions.Item>
              <Descriptions.Item label="复查人">{detail.reviewer_id ? userName(detail.reviewer_id) : '-'}</Descriptions.Item>
              <Descriptions.Item label="整改说明" span={2}>{detail.rectification_note || '-'}</Descriptions.Item>
              {detail.rectification_photo && (
                <Descriptions.Item label="整改照片" span={2}>
                  <Image src={detail.rectification_photo} width={180} />
                </Descriptions.Item>
              )}
              {detail.review_note && (
                <Descriptions.Item label="复查意见" span={2}>
                  <span style={{ color: detail.status === RectTaskStatus.REJECTED ? '#cf1322' : undefined }}>
                    {detail.review_note}（{formatDateTime(detail.reviewed_at)}）
                  </span>
                </Descriptions.Item>
              )}
            </Descriptions>
            <div>
              <div style={{ fontWeight: 600, marginBottom: 8 }}>办理记录</div>
              <Timeline
                items={histories.map((h) => ({
                  color: RectHistoryActionColor[h.action] || 'gray',
                  children: (
                    <div>
                      <Space size={8}>
                        <Tag color={RectHistoryActionColor[h.action]}>{RectHistoryActionText[h.action] || h.action}</Tag>
                        <span style={{ color: '#999', fontSize: 12 }}>
                          {userName(h.operator_id)} · {formatDateTime(h.created_at)}
                        </span>
                      </Space>
                      {h.note && <div style={{ marginTop: 4 }}>{h.note}</div>}
                      {h.photo_url && <Image src={h.photo_url} width={140} style={{ marginTop: 4 }} />}
                    </div>
                  ),
                }))}
              />
            </div>
          </Space>
        )}
      </Modal>
    </Card>
  )
}