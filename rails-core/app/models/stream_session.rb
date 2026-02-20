class StreamSession < ApplicationRecord
  belongs_to :company
  belongs_to :device
  belongs_to :user

  enum status: { requested: 0, active: 1, ended: 2, failed: 3 }, _default: :requested

  validates :session_id, :stream_name, presence: true
  validates :session_id, uniqueness: true
  validates :channel, numericality: { greater_than: 0, less_than_or_equal_to: 16 }
end
