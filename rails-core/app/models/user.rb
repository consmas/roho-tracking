class User < ApplicationRecord
  belongs_to :company
  has_many :commands, dependent: :nullify

  has_secure_password

  enum role: { viewer: 0, operator: 1, admin: 2 }, _default: :viewer

  validates :email, presence: true, uniqueness: true
end
