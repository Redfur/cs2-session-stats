import { useState } from 'react'
import { useNavigate } from 'react-router'
import { api } from '../api'
import { paths, useGroups } from '../groups'
import { FormDialog } from './ui/Dialog'
import { Field, Input } from './ui/Field'

export const MAX_GROUP_NAME = 100

// NewGroupDialog создаёт группу и открывает её пустой список сессий.
export function NewGroupDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  const [name, setName] = useState('')
  const [tried, setTried] = useState(false)
  const { reload } = useGroups()
  const navigate = useNavigate()
  const [prevOpen, setPrevOpen] = useState(open)
  if (open !== prevOpen) {
    setPrevOpen(open)
    if (open) {
      setName('')
      setTried(false)
    }
  }
  const empty = tried && !name.trim()
  return (
    <FormDialog
      open={open}
      onClose={onClose}
      title="Новая группа"
      submitLabel="Создать группу"
      errorTitle="Не удалось создать группу."
      validate={() => {
        setTried(true)
        return name.trim() !== ''
      }}
      onSubmit={async () => {
        const g = await api.createGroup(name)
        await reload().catch(() => {})
        navigate(paths.sessions(g.id))
      }}
    >
      <p className="m-0 -mt-2 text-[13px] leading-[19px] text-fg-muted">
        У группы свои сессии, игроки и рейтинг. OVR считается только по матчам этой группы.
      </p>
      <Field
        label="Название"
        htmlFor="ng-name"
        error={empty && 'Введите название группы'}
        help={
          <span className="tabular-nums">
            {name.length} / {MAX_GROUP_NAME}
          </span>
        }
      >
        <Input
          id="ng-name"
          type="text"
          maxLength={MAX_GROUP_NAME}
          placeholder="Например, «Четверг»"
          autoFocus
          value={name}
          aria-invalid={empty || undefined}
          onChange={(e) => {
            setName(e.target.value)
            setTried(false)
          }}
        />
      </Field>
    </FormDialog>
  )
}
