class Alarm < ApplicationRecord
  belongs_to :company
  belongs_to :device
  belongs_to :event

  enum severity: { low: 0, medium: 1, high: 2, critical: 3 }, _default: :medium

  validates :alarm_type, :occurred_at, presence: true
end
