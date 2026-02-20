class Command < ApplicationRecord
  COMMAND_TYPES = %w[reboot snapshot config_update].freeze

  belongs_to :company
  belongs_to :device
  belongs_to :user

  enum status: { queued: 0, delivered: 1, acknowledged: 2, failed: 3 }, _default: :queued

  validates :command_id, :command_type, :requested_at, presence: true
  validates :command_type, inclusion: { in: COMMAND_TYPES }
end
