class StreamSession < ApplicationRecord
  belongs_to :company
  belongs_to :device
  belongs_to :user

  enum status: { requested: 0, active: 1, ended: 2, failed: 3 }, _default: :requested

  validates :session_id, :stream_name, presence: true
  validates :session_id, uniqueness: true
  validates :channel, numericality: { greater_than: 0, less_than_or_equal_to: 16 }

  scope :open_states, -> { where(status: [:requested, :active]) }

  def self.find_open(device_id:, channel:)
    open_states.where(device_id:, channel:).order(created_at: :desc).first
  end
end
